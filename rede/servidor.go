// Grupo 10 — Redes de Computadores
// Fabio Willian Alves Silva, 251020487
// Gustavo Vieira de Araujo, 211068440
// Joao Francisco de Sousa Torres, 251037072

package rede

import (
	"encoding/json"
	"fmt"
	"net"
	"time"

	"cliente-p2p/configuracao"
	"cliente-p2p/peer"
	"cliente-p2p/protocolo"
	"cliente-p2p/registro"
	"cliente-p2p/roteador"
)

// Servidor aceita conexões TCP entrantes de outros peers e realiza o
// handshake HELLO/HELLO_OK antes de iniciar o leitor e o keep-alive.
type Servidor struct {
	cfg         *configuracao.Configuracao
	gerenciador *peer.GerenciadorConexoes
	roteador    *roteador.Roteador
}

// NovoServidor cria um Servidor associado à configuração, ao gerenciador de
// conexões e ao roteador do peer local.
func NovoServidor(cfg *configuracao.Configuracao, gerenciador *peer.GerenciadorConexoes, rot *roteador.Roteador) *Servidor {
	return &Servidor{cfg: cfg, gerenciador: gerenciador, roteador: rot}
}

// Iniciar abre um socket TCP em 0.0.0.0:porta e aceita conexões em uma
// goroutine separada.
func (s *Servidor) Iniciar() error {
	endereco := fmt.Sprintf("0.0.0.0:%d", s.cfg.Porta)
	escuta, err := net.Listen("tcp", endereco)
	if err != nil {
		return err
	}
	registro.Informar("Servidor", "escutando em %s", endereco)
	go func() {
		for {
			conexaoTCP, err := escuta.Accept()
			if err != nil {
				registro.Erro("Servidor", "accept: %v", err)
				continue
			}
			go s.tratarConexaoEntrante(conexaoTCP)
		}
	}()
	return nil
}

// tratarConexaoEntrante realiza o handshake HELLO/HELLO_OK com um peer que
// se conectou a nós e, em caso de sucesso, registra a conexão e inicia o
// leitor e o keep-alive.
func (s *Servidor) tratarConexaoEntrante(conexaoTCP net.Conn) {
	conexaoTCP.SetDeadline(time.Now().Add(10 * time.Second))

	conexao := peer.NovaConexaoPeer(conexaoTCP.RemoteAddr().String(), conexaoTCP, peer.DirecaoEntrada)

	linha, err := conexao.LerLinha()
	if err != nil {
		registro.Erro("Servidor", "falha ao ler HELLO de %s: %v", conexaoTCP.RemoteAddr(), err)
		conexaoTCP.Close()
		return
	}

	var hello protocolo.MensagemHello
	if err := json.Unmarshal(linha, &hello); err != nil || hello.Tipo != "HELLO" {
		registro.Erro("Servidor", "HELLO inválido de %s", conexaoTCP.RemoteAddr())
		conexaoTCP.Close()
		return
	}

	conexao.IDPeer = hello.IDPeer
	conexaoTCP.SetDeadline(time.Time{})

	helloOk := protocolo.MensagemHello{
		Tipo:     "HELLO_OK",
		IDPeer:   s.cfg.MeuID(),
		Versao:   "1.0",
		Recursos: []string{"ack", "metrics"},
		TTL:      1,
	}
	if err := conexao.EscreverJSON(helloOk); err != nil {
		registro.Erro("Servidor", "falha ao enviar HELLO_OK para %s: %v", hello.IDPeer, err)
		conexaoTCP.Close()
		return
	}

	registro.Informar("Servidor", "inbound conectado: %s", conexao.IDPeer)

	// Fecha conexão antiga com o mesmo peer, se existir.
	if antiga, ok := s.gerenciador.Obter(conexao.IDPeer); ok {
		antiga.Fechar()
		s.gerenciador.Remover(conexao.IDPeer)
	}

	s.gerenciador.Adicionar(conexao)
	go iniciarManutencaoConexao(conexao, s.cfg)
	go iniciarLeitor(conexao, s.gerenciador, s.roteador, s.cfg)
}
