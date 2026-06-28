// Grupo 10 — Redes de Computadores
// Fabio Willian Alves Silva, 251020487
// Gustavo Vieira de Araujo, 211068440
// Joao Francisco de Sousa Torres, 251037072

// Package cli implementa a interface de linha de comando interativa do
// cliente P2P (/peers, /msg, /pub, /conn, /rtt, /reconnect, /log, /quit).
package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"cliente-p2p/configuracao"
	"cliente-p2p/peer"
	"cliente-p2p/protocolo"
	"cliente-p2p/rede"
	"cliente-p2p/registro"
	"cliente-p2p/rendezvous"
	"cliente-p2p/roteador"
)

// CLI agrupa as dependências necessárias para executar os comandos
// digitados pelo usuário.
type CLI struct {
	cfg         *configuracao.Configuracao
	tabela      *peer.TabelaPeers
	gerenciador *peer.GerenciadorConexoes
	rdv         *rendezvous.ClienteRendezvous
	roteador    *roteador.Roteador
	conector    *rede.Conector
}

// NovaCLI cria a interface de linha de comando associada às dependências do
// peer local.
func NovaCLI(cfg *configuracao.Configuracao, tabela *peer.TabelaPeers, gerenciador *peer.GerenciadorConexoes, rdv *rendezvous.ClienteRendezvous, rot *roteador.Roteador, conector *rede.Conector) *CLI {
	return &CLI{cfg: cfg, tabela: tabela, gerenciador: gerenciador, rdv: rdv, roteador: rot, conector: conector}
}

// Executar imprime o prompt e processa os comandos digitados pelo usuário
// até o final da entrada padrão (ou até /quit).
func (c *CLI) Executar() {
	fmt.Printf("Conectado como %s\n", c.cfg.MeuID())
	fmt.Println("Comandos: /peers, /msg, /pub, /conn, /rtt, /reconnect, /log, /quit")
	fmt.Print("> ")

	leitor := bufio.NewScanner(os.Stdin)
	for leitor.Scan() {
		linha := strings.TrimSpace(leitor.Text())
		if linha != "" {
			c.processar(linha)
		}
		fmt.Print("> ")
	}
}

// processar interpreta uma linha digitada e despacha para o comando
// correspondente.
func (c *CLI) processar(linha string) {
	partes := strings.Fields(linha)
	if len(partes) == 0 {
		return
	}

	comando := partes[0]
	args := partes[1:]

	switch comando {
	case "/peers":
		c.comandoPeers(args)
	case "/msg":
		c.comandoMsg(args)
	case "/pub":
		c.comandoPub(args)
	case "/conn":
		c.comandoConexoes()
	case "/rtt":
		c.comandoRTT()
	case "/reconnect":
		c.comandoReconectar()
	case "/log":
		c.comandoLog(args)
	case "/quit":
		c.comandoSair()
	default:
		fmt.Printf("Comando desconhecido: %s\n", comando)
		fmt.Println("Comandos: /peers, /msg, /pub, /conn, /rtt, /reconnect, /log, /quit")
	}
}

// comandoPeers executa DISCOVER no Rendezvous e lista os peers encontrados.
// args[0] pode ser "*" (todos os namespaces) ou "#namespace" (namespace
// específico); se omitido, usa o namespace do peer local.
func (c *CLI) comandoPeers(args []string) {
	namespace := c.cfg.Namespace
	if len(args) > 0 {
		if args[0] == "*" {
			namespace = ""
		} else {
			namespace = strings.TrimPrefix(args[0], "#")
		}
	}

	peers, err := c.rdv.Descobrir(namespace)
	if err != nil {
		fmt.Printf("Erro: %v\n", err)
		return
	}

	meuID := c.cfg.MeuID()
	encontrados := 0
	for _, p := range peers {
		if p.Identificador() == meuID {
			continue
		}
		conectado := ""
		if c.gerenciador.Possui(p.Identificador()) {
			conectado = " [conectado]"
		}
		fmt.Printf("  %s  %s:%d  expira_em=%ds%s\n",
			p.Identificador(), p.IP, p.Porta, p.ExpiraEm, conectado)

		// Atualiza a tabela local com os dados frescos do Rendezvous.
		p.Estado = protocolo.EstadoAtivo
		c.tabela.Atualizar(p)
		encontrados++
	}

	if encontrados == 0 {
		fmt.Println("  Nenhum peer encontrado.")
	}
	c.conector.Disparar()
}

// comandoMsg envia uma mensagem direta (SEND) para o peer indicado.
// Uso: /msg <peer_id> <mensagem>
func (c *CLI) comandoMsg(args []string) {
	if len(args) < 2 {
		fmt.Println("Uso: /msg <peer_id> <mensagem>")
		return
	}
	idPeer := args[0]
	conteudo := strings.Join(args[1:], " ")
	if err := c.roteador.Enviar(idPeer, conteudo); err != nil {
		fmt.Printf("Erro: %v\n", err)
	}
}

// comandoPub envia uma mensagem de difusão (PUB).
// Uso: /pub * <mensagem>  |  /pub #<namespace> <mensagem>
func (c *CLI) comandoPub(args []string) {
	if len(args) < 2 {
		fmt.Println("Uso: /pub * <mensagem>  |  /pub #<namespace> <mensagem>")
		return
	}
	destino := args[0]
	conteudo := strings.Join(args[1:], " ")
	c.roteador.Publicar(destino, conteudo)
	fmt.Printf("PUB enviado para %s\n", destino)
}

// comandoConexoes lista as conexões P2P ativas, sua direção (entrada/saída)
// e o RTT medido.
func (c *CLI) comandoConexoes() {
	conexoes := c.gerenciador.Listar()
	if len(conexoes) == 0 {
		fmt.Println("  Nenhuma conexão ativa.")
		return
	}
	for _, conexao := range conexoes {
		rtt := conexao.ObterRTT()
		rttTexto := "sem dados"
		if rtt > 0 {
			rttTexto = fmt.Sprintf("%.1fms", rtt)
		}
		fmt.Printf("  %s  (%s)  rtt=%s\n", conexao.IDPeer, conexao.Direcao, rttTexto)
	}
}

// comandoRTT exibe o RTT médio medido para cada conexão ativa.
func (c *CLI) comandoRTT() {
	conexoes := c.gerenciador.Listar()
	if len(conexoes) == 0 {
		fmt.Println("  Nenhuma conexão ativa.")
		return
	}
	for _, conexao := range conexoes {
		rtt := conexao.ObterRTT()
		if rtt == 0 {
			fmt.Printf("  %s: sem dados de RTT ainda\n", conexao.IDPeer)
		} else {
			fmt.Printf("  %s: RTT médio = %.1f ms\n", conexao.IDPeer, rtt)
		}
	}
}

// comandoReconectar executa DISCOVER imediatamente para atualizar a tabela
// de peers e depois dispara o conector para estabelecer conexões pendentes,
// sem esperar o próximo ciclo automático de 60s.
func (c *CLI) comandoReconectar() {
	fmt.Println("Forçando descoberta e reconexão...")
	peers, err := c.rdv.Descobrir(c.cfg.Namespace)
	if err != nil {
		fmt.Printf("Aviso: DISCOVER falhou (%v) — reconciliando com peers já conhecidos\n", err)
	} else {
		meuID := c.cfg.MeuID()
		c.tabela.MarcarTodosObsoletos()
		for _, p := range peers {
			if p.Identificador() == meuID {
				continue
			}
			p.Estado = protocolo.EstadoAtivo
			c.tabela.Atualizar(p)
		}
	}
	c.conector.Disparar()
}

// comandoLog altera o nível de log em tempo de execução.
// Uso: /log <DEBUG|INFO|WARN|ERROR>
func (c *CLI) comandoLog(args []string) {
	if len(args) == 0 {
		fmt.Println("Uso: /log <DEBUG|INFO|WARN|ERROR>")
		return
	}
	registro.DefinirNivel(args[0])
	fmt.Printf("Nível de log: %s\n", strings.ToUpper(args[0]))
}

// comandoSair envia BYE para todos os peers conectados, desregistra o peer
// local do Rendezvous e encerra o programa.
func (c *CLI) comandoSair() {
	fmt.Println("Encerrando...")
	for _, conexao := range c.gerenciador.Listar() {
		rede.EnviarTchau(conexao, c.cfg, 2*time.Second)
	}
	_ = c.rdv.Desregistrar()
	os.Exit(0)
}
