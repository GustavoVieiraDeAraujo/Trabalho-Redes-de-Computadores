// Grupo 10 — Redes de Computadores
// Fabio Willian Alves Silva, 251020487
// Gustavo Vieira de Araujo, 211068440
// Joao Francisco de Sousa Torres, 251037072

package rede

import (
	"encoding/json"
	"net"
	"strconv"
	"sync"
	"time"

	"cliente-p2p/configuracao"
	"cliente-p2p/peer"
	"cliente-p2p/protocolo"
	"cliente-p2p/registro"
	"cliente-p2p/roteador"
)

// Conector estabelece conexões TCP de saída com peers marcados como ATIVO
// na TabelaPeers que ainda não possuem conexão, com backoff exponencial e
// sem duplicar tentativas concorrentes para o mesmo peer.
type Conector struct {
	cfg         *configuracao.Configuracao
	tabela      *peer.TabelaPeers
	gerenciador *peer.GerenciadorConexoes
	roteador    *roteador.Roteador

	mutexConectando sync.Mutex
	conectando      map[string]bool

	canalDisparo chan struct{}
}

// NovoConector cria um Conector associado à configuração, tabela de peers,
// gerenciador de conexões e roteador do peer local.
func NovoConector(cfg *configuracao.Configuracao, tabela *peer.TabelaPeers, gerenciador *peer.GerenciadorConexoes, rot *roteador.Roteador) *Conector {
	return &Conector{
		cfg:          cfg,
		tabela:       tabela,
		gerenciador:  gerenciador,
		roteador:     rot,
		conectando:   make(map[string]bool),
		canalDisparo: make(chan struct{}, 1),
	}
}

// Disparar força uma reconciliação imediata (não bloqueia se já houver uma
// reconciliação pendente).
func (c *Conector) Disparar() {
	select {
	case c.canalDisparo <- struct{}{}:
	default:
	}
}

// Iniciar roda o laço principal do conector: reconcilia a tabela de peers a
// cada 30 segundos ou sempre que Disparar for chamado.
func (c *Conector) Iniciar() {
	time.Sleep(2 * time.Second)
	c.reconciliar()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.reconciliar()
		case <-c.canalDisparo:
			c.reconciliar()
		}
	}
}

// reconciliar percorre a tabela de peers e dispara uma goroutine de conexão
// para cada peer ATIVO que ainda não tem conexão estabelecida.
func (c *Conector) reconciliar() {
	meuID := c.cfg.MeuID()
	for _, p := range c.tabela.Listar() {
		if p.Identificador() == meuID || p.Estado != protocolo.EstadoAtivo {
			continue
		}
		if c.gerenciador.Possui(p.Identificador()) {
			continue
		}

		c.mutexConectando.Lock()
		jaConectando := c.conectando[p.Identificador()]
		if !jaConectando {
			c.conectando[p.Identificador()] = true
		}
		c.mutexConectando.Unlock()

		if !jaConectando {
			go func(peerAlvo protocolo.RegistroPeer) {
				defer func() {
					c.mutexConectando.Lock()
					delete(c.conectando, peerAlvo.Identificador())
					c.mutexConectando.Unlock()
				}()
				c.conectarAoPeer(peerAlvo)
			}(p)
		}
	}
}

// conectarAoPeer tenta estabelecer uma conexão de saída com peerAlvo,
// realizando o handshake HELLO/HELLO_OK. Em caso de falha, tenta novamente
// com backoff exponencial (1s, 2s, 4s, ...) até MaxTentativasReconexao.
func (c *Conector) conectarAoPeer(peerAlvo protocolo.RegistroPeer) {
	espera := time.Second
	maximo := c.cfg.MaxTentativasReconexao

	for tentativa := 1; tentativa <= maximo; tentativa++ {
		if c.gerenciador.Possui(peerAlvo.Identificador()) {
			return
		}

		endereco := net.JoinHostPort(peerAlvo.IP, strconv.Itoa(peerAlvo.Porta))
		conexaoTCP, err := net.DialTimeout("tcp", endereco, 5*time.Second)
		if err != nil {
			registro.Alertar("Conector", "tentativa %d/%d para %s falhou: %v", tentativa, maximo, peerAlvo.Identificador(), err)
			if tentativa < maximo {
				time.Sleep(espera)
				espera *= 2
			}
			continue
		}

		conexao := peer.NovaConexaoPeer(peerAlvo.Identificador(), conexaoTCP, peer.DirecaoSaida)
		conexaoTCP.SetDeadline(time.Now().Add(10 * time.Second))

		hello := protocolo.MensagemHello{
			Tipo:     "HELLO",
			IDPeer:   c.cfg.MeuID(),
			Versao:   "1.0",
			Recursos: []string{"ack", "metrics"},
			TTL:      1,
		}
		if err := conexao.EscreverJSON(hello); err != nil {
			registro.Alertar("Conector", "HELLO falhou para %s: %v", peerAlvo.Identificador(), err)
			conexaoTCP.Close()
			time.Sleep(espera)
			espera *= 2
			continue
		}

		linha, err := conexao.LerLinha()
		if err != nil {
			registro.Alertar("Conector", "HELLO_OK falhou para %s: %v", peerAlvo.Identificador(), err)
			conexaoTCP.Close()
			time.Sleep(espera)
			espera *= 2
			continue
		}

		var resp protocolo.MensagemHello
		if err := json.Unmarshal(linha, &resp); err != nil || resp.Tipo != "HELLO_OK" {
			registro.Alertar("Conector", "HELLO_OK inválido de %s", peerAlvo.Identificador())
			conexaoTCP.Close()
			time.Sleep(espera)
			espera *= 2
			continue
		}

		conexaoTCP.SetDeadline(time.Time{})
		registro.Informar("Conector", "conectado a %s", peerAlvo.Identificador())
		c.gerenciador.Adicionar(conexao)
		c.tabela.MarcarAtivo(peerAlvo.Identificador())
		go iniciarManutencaoConexao(conexao, c.cfg)
		go iniciarLeitor(conexao, c.gerenciador, c.roteador, c.cfg)
		return
	}

	registro.Alertar("Conector", "desistindo de %s após %d tentativas", peerAlvo.Identificador(), maximo)
}
