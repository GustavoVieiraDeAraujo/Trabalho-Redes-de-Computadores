// Grupo 10 — Redes de Computadores
// Fabio Willian Alves Silva, 251020487
// Gustavo Vieira de Araujo, 211068440
// Joao Francisco de Sousa Torres, 251037072

// Package cli implementa a interface de linha de comando interativa usando
// o TUI bubbletea com tela dividida: viewport de mensagens (rolável) acima e
// campo de entrada sempre visível abaixo. Suporta histórico (↑↓), numeração
// de peers e saída colorida com timestamps.
package cli

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"cliente-p2p/configuracao"
	"cliente-p2p/peer"
	"cliente-p2p/protocolo"
	"cliente-p2p/rede"
	"cliente-p2p/registro"
	"cliente-p2p/rendezvous"
	"cliente-p2p/roteador"
	"cliente-p2p/ui"
)

var comandos = []string{
	"/peers", "/msg", "/pub", "/conn", "/reconnect", "/help", "/quit",
}

// CLI agrupa as dependências necessárias para executar os comandos interativos.
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
	return &CLI{
		cfg:         cfg,
		tabela:      tabela,
		gerenciador: gerenciador,
		rdv:         rdv,
		roteador:    rot,
		conector:    conector,
	}
}

// Executar inicia o TUI bubbletea.
func (c *CLI) Executar() {
	registro.DefinirNivel("WARN")
	if err := iniciarTUI(c); err != nil {
		fmt.Fprintf(os.Stderr, "erro no TUI: %v\n", err)
		os.Exit(1)
	}
}

// cleanup envia BYE para todos os peers em paralelo e desregistra do
// Rendezvous. Chamado imediatamente antes de encerrar o TUI.
func (c *CLI) cleanup() {
	var wg sync.WaitGroup
	for _, conexao := range c.gerenciador.Listar() {
		wg.Add(1)
		go func(conn *peer.ConexaoPeer) {
			defer wg.Done()
			rede.EnviarTchau(conn, c.cfg, 2*time.Second)
		}(conexao)
	}
	wg.Wait()
	_ = c.rdv.Desregistrar()
}

func (c *CLI) processar(linha string) {
	partes := strings.Fields(linha)
	if len(partes) == 0 {
		return
	}
	switch partes[0] {
	case "/peers":
		c.comandoPeers(partes[1:])
	case "/msg":
		c.comandoMsg(partes[1:])
	case "/pub":
		c.comandoPub(partes[1:])
	case "/conn":
		c.comandoConexoes()
	case "/reconnect":
		c.comandoReconectar()
	case "/help":
		c.comandoAjuda()
	case "/quit":
		c.comandoSair()
	default:
		ui.Erro("comando desconhecido: %s  (use /help)", partes[0])
	}
}

// comandoPeers executa DISCOVER, numera os peers e exibe status de conexão.
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
		ui.Erro("DISCOVER falhou: %v", err)
		return
	}

	ui.EnviarMsg(ui.MsgUI{Tipo: "cmd_sep", Conteudo: "/peers"})
	meuID := c.cfg.MeuID()
	labelNS := namespace
	if labelNS == "" {
		labelNS = "todos"
	}
	ui.Linha("\033[1mPeers — namespace '%s':\033[0m", labelNS)

	encontrados := 0
	for _, p := range peers {
		if p.Identificador() == meuID {
			continue
		}
		num := ui.RegistrarPeer(p.Identificador())
		p.Estado = protocolo.EstadoAtivo
		c.tabela.Atualizar(p)

		status := "\033[90mdesconectado\033[0m"
		if c.gerenciador.Possui(p.Identificador()) {
			status = "\033[32mconectado\033[0m"
		}
		ui.Linha("\033[1m[%d]\033[0m %-28s %s:%d  expira=%ds  %s",
			num, p.Identificador(), p.IP, p.Porta, p.ExpiraEm, status)
		encontrados++
	}

	if encontrados == 0 {
		ui.Linha("Nenhum peer encontrado.")
	} else {
		ui.Linha("\033[90mUse /msg <numero> <mensagem> para enviar.\033[0m")
	}
	c.conector.Disparar()
}

// comandoMsg envia mensagem direta, aceitando número ou ID completo.
func (c *CLI) comandoMsg(args []string) {
	if len(args) < 2 {
		ui.Erro("uso: /msg <peer_id|numero> <mensagem>")
		return
	}
	idPeer, ok := ui.ResolverPeer(args[0])
	if !ok {
		ui.Erro("peer #%s não encontrado — use /peers para atualizar a lista", args[0])
		return
	}
	conteudo := strings.Join(args[1:], " ")
	if err := c.roteador.Enviar(idPeer, conteudo); err != nil {
		ui.Erro("%v", err)
	} else {
		ui.EnviarMsg(ui.MsgUI{Tipo: "enviado", Destino: idPeer, Conteudo: conteudo})
	}
}

func (c *CLI) comandoPub(args []string) {
	if len(args) < 2 {
		ui.Erro("uso: /pub * <mensagem>  |  /pub #<namespace> <mensagem>")
		return
	}
	destino := args[0]
	conteudo := strings.Join(args[1:], " ")
	c.roteador.Publicar(destino, conteudo)
	ui.EnviarMsg(ui.MsgUI{Tipo: "cmd_sep", Conteudo: "/pub"})
	ui.Confirmacao("PUB enviado para %s", destino)
}

// comandoConexoes lista conexões ativas com IP, RTT e direção.
func (c *CLI) comandoConexoes() {
	ui.EnviarMsg(ui.MsgUI{Tipo: "cmd_sep", Conteudo: "/conn"})
	conexoes := c.gerenciador.Listar()
	if len(conexoes) == 0 {
		ui.Linha("Nenhuma conexão ativa.")
		return
	}
	ui.Linha("\033[1mConexões ativas:\033[0m")
	for _, conexao := range conexoes {
		rtt := conexao.ObterRTT()
		rttTexto := "\033[90msem dados\033[0m"
		if rtt > 0 {
			rttTexto = fmt.Sprintf("\033[32m%.1fms\033[0m", rtt)
		}
		num := ui.RegistrarPeer(conexao.IDPeer)
		ui.Linha("\033[1m[%d]\033[0m %-28s %-16s %-8s há %-8s RTT=%s",
			num, conexao.IDPeer, conexao.IP, conexao.Direcao,
			duracaoConexao(conexao.ConectadoEm), rttTexto)
	}
}

func duracaoConexao(t time.Time) string {
	if t.IsZero() {
		return "?"
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dmin", int(d.Minutes()))
	default:
		return fmt.Sprintf("%dh%dmin", int(d.Hours()), int(d.Minutes())%60)
	}
}

func (c *CLI) comandoReconectar() {
	ui.EnviarMsg(ui.MsgUI{Tipo: "cmd_sep", Conteudo: "/reconnect"})
	ui.Linha("Forçando descoberta e reconexão...")
	peers, err := c.rdv.Descobrir(c.cfg.Namespace)
	if err != nil {
		ui.Erro("DISCOVER falhou: %v — reconciliando com peers conhecidos", err)
	} else {
		meuID := c.cfg.MeuID()
		c.tabela.MarcarTodosObsoletos()
		for _, p := range peers {
			if p.Identificador() == meuID {
				continue
			}
			p.Estado = protocolo.EstadoAtivo
			c.tabela.Atualizar(p)
			ui.RegistrarPeer(p.Identificador())
		}
	}
	c.conector.Disparar()
	ui.Confirmacao("Reconexão disparada.")
}

func (c *CLI) comandoAjuda() {
	const (
		rst = "\033[0m"
		cmd = "\033[1;96m"     // ciano brilhante — comando
		arg = "\033[38;5;245m" // cinza médio — argumentos
		ex  = "\033[92m"       // verde — exemplo
		hdr = "\033[38;5;243m" // cinza — cabeçalho
		div = "\033[38;5;236m" // cinza escuro — divisória
	)

	linha := func(nome, sintaxe, exemplo string) string {
		return fmt.Sprintf("%s%-13s%s %s%-32s%s %s%s%s",
			cmd, nome, rst,
			arg, sintaxe, rst,
			ex, exemplo, rst)
	}

	bloco := strings.Join([]string{
		fmt.Sprintf("%s%-13s %-32s %s%s", hdr, "Comando", "Argumentos", "Exemplo", rst),
		fmt.Sprintf("%s%-13s %-32s %s%s", div,
			strings.Repeat("─", 11), strings.Repeat("─", 30), strings.Repeat("─", 18), rst),
		linha("/peers", "[* | #namespace]", "/peers *"),
		linha("/msg", "<id|numero> <mensagem>", "/msg 1 exemplo"),
		linha("/pub", "<* | #namespace> <mensagem>", "/pub * exemplo"),
		linha("/conn", "", "/conn"),
		linha("/reconnect", "", "/reconnect"),
		linha("/help", "", "/help"),
		linha("/quit", "", "/quit"),
	}, "\n")

	ui.EnviarMsg(ui.MsgUI{Tipo: "cmd_sep", Conteudo: "/help"})
	ui.EnviarMsg(ui.MsgUI{Tipo: "info", Conteudo: bloco})
}

// comandoSair sinaliza ao TUI para encerrar de forma limpa (BYE + unregister).
func (c *CLI) comandoSair() {
	ui.EnviarMsg(ui.MsgUI{Tipo: "quit"})
}
