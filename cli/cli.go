// Grupo 10 — Redes de Computadores
// Fabio Willian Alves Silva, 251020487
// Gustavo Vieira de Araujo, 211068440
// Joao Francisco de Sousa Torres, 251037072

// Package cli implementa a interface de linha de comando interativa com
// histórico de comandos (↑↓), tab-completion, numeração de peers e saída
// colorida com timestamps.
package cli

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/peterh/liner"

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
	"/peers", "/msg", "/pub", "/conn", "/reconnect", "/log", "/help", "/quit",
}

// CLI agrupa as dependências necessárias para executar os comandos interativos.
type CLI struct {
	cfg         *configuracao.Configuracao
	tabela      *peer.TabelaPeers
	gerenciador *peer.GerenciadorConexoes
	rdv         *rendezvous.ClienteRendezvous
	roteador    *roteador.Roteador
	conector    *rede.Conector
	liner       *liner.State
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

// Executar inicializa o liner e entra no loop principal de leitura de
// comandos. Suprime logs de nível INFO para não poluir o chat.
func (c *CLI) Executar() {
	registro.DefinirNivel("WARN")

	l := liner.NewLiner()
	defer l.Close()
	l.SetCtrlCAborts(true)
	c.liner = l

	// Tab-completion: completa comandos e IDs de peers após /msg.
	l.SetCompleter(func(line string) (comp []string) {
		partes := strings.Fields(line)
		if len(partes) <= 1 && strings.HasPrefix(line, "/") {
			prefix := line
			for _, cmd := range comandos {
				if strings.HasPrefix(cmd, prefix) {
					comp = append(comp, cmd+" ")
				}
			}
			return
		}
		if len(partes) == 2 && partes[0] == "/msg" {
			for _, id := range ui.ListarIDsPeers() {
				if strings.HasPrefix(id, partes[1]) {
					comp = append(comp, "/msg "+id+" ")
				}
			}
		}
		return
	})

	prompt := fmt.Sprintf("[%s] > ", c.cfg.MeuID())
	ui.DefinirPrompt(prompt)

	fmt.Printf("\033[1m\033[32mConectado como %s\033[0m\n", c.cfg.MeuID())
	fmt.Println("Digite \033[1m/help\033[0m para ver os comandos disponíveis.")

	for {
		texto, err := l.Prompt(prompt)
		if err == liner.ErrPromptAborted || err == io.EOF {
			c.comandoSair()
			return
		}
		texto = strings.TrimSpace(texto)
		if texto != "" {
			l.AppendHistory(texto)
			c.processar(texto)
		}
	}
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
	case "/rtt":
		c.comandoConexoes()
	case "/reconnect":
		c.comandoReconectar()
	case "/log":
		c.comandoLog(partes[1:])
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
		ui.Linha("  \033[1m[%d]\033[0m %-28s %s:%d  expira=%ds  %s",
			num, p.Identificador(), p.IP, p.Porta, p.ExpiraEm, status)
		encontrados++
	}

	if encontrados == 0 {
		ui.Linha("  Nenhum peer encontrado.")
	} else {
		ui.Linha("  \033[90mUse /msg <numero> <mensagem> para enviar.\033[0m")
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
	ui.Confirmacao("PUB enviado para %s", destino)
}

// comandoConexoes lista conexões ativas com RTT e direção (unifica /conn e /rtt).
func (c *CLI) comandoConexoes() {
	conexoes := c.gerenciador.Listar()
	if len(conexoes) == 0 {
		ui.Linha("  Nenhuma conexão ativa.")
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
		ui.Linha("  \033[1m[%d]\033[0m %-28s (%s)  RTT=%s",
			num, conexao.IDPeer, conexao.Direcao, rttTexto)
	}
}

func (c *CLI) comandoReconectar() {
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

func (c *CLI) comandoLog(args []string) {
	if len(args) == 0 {
		ui.Erro("uso: /log <DEBUG|INFO|WARN|ERROR>")
		return
	}
	nivel := strings.ToUpper(args[0])
	registro.DefinirNivel(nivel)
	ui.Confirmacao("Nível de log alterado para %s", nivel)
}

func (c *CLI) comandoAjuda() {
	ui.Linha("\033[1mComandos disponíveis:\033[0m")
	ui.Linha("")
	ui.Linha("  \033[1m/peers [* | #namespace]\033[0m")
	ui.Linha("    Lista peers no Rendezvous e atribui números de atalho.")
	ui.Linha("    sem argumento = namespace atual  |  * = todos  |  #ns = específico")
	ui.Linha("")
	ui.Linha("  \033[1m/msg <id|numero> <mensagem>\033[0m")
	ui.Linha("    Envia mensagem direta. Use o número exibido em /peers ou /conn.")
	ui.Linha("")
	ui.Linha("  \033[1m/pub <* | #namespace> <mensagem>\033[0m")
	ui.Linha("    Difunde mensagem para todos (*) ou somente para um namespace.")
	ui.Linha("")
	ui.Linha("  \033[1m/conn\033[0m")
	ui.Linha("    Lista conexões TCP ativas com direção e RTT medido.")
	ui.Linha("")
	ui.Linha("  \033[1m/reconnect\033[0m")
	ui.Linha("    Força DISCOVER imediato e tenta conectar a todos os peers ativos.")
	ui.Linha("")
	ui.Linha("  \033[1m/log <DEBUG|INFO|WARN|ERROR>\033[0m")
	ui.Linha("    Altera o nível de log em tempo real.")
	ui.Linha("")
	ui.Linha("  \033[1m/help\033[0m  — esta tela")
	ui.Linha("  \033[1m/quit\033[0m  — encerra (envia BYE, desregistra do Rendezvous)")
	ui.Linha("")
	ui.Linha("  \033[90mDica: TAB completa comandos e IDs. ↑↓ navega no histórico.\033[0m")
}

func (c *CLI) comandoSair() {
	fmt.Println("\nEncerrando...")
	for _, conexao := range c.gerenciador.Listar() {
		rede.EnviarTchau(conexao, c.cfg, 2*time.Second)
	}
	_ = c.rdv.Desregistrar()
	if c.liner != nil {
		c.liner.Close()
	}
	os.Exit(0)
}
