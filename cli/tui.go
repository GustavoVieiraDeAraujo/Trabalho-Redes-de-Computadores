// Grupo 10 — Redes de Computadores
// Fabio Willian Alves Silva, 251020487
// Gustavo Vieira de Araujo, 211068440
// Joao Francisco de Sousa Torres, 251037072

package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"cliente-p2p/registro"
	"cliente-p2p/ui"
)

// --- Estilos lipgloss ---

var (
	corAzul     = lipgloss.Color("63")
	corCiano    = lipgloss.Color("45")
	corAmarelo  = lipgloss.Color("214")
	corVerde    = lipgloss.Color("76")
	corVermelho = lipgloss.Color("196")
	corCinza    = lipgloss.Color("240")

	styleStatusBar = lipgloss.NewStyle().
			Background(corAzul).
			Foreground(lipgloss.Color("230")).
			Padding(0, 1)

	styleSep = lipgloss.NewStyle().Foreground(lipgloss.Color("238"))

	styleTimestamp  = lipgloss.NewStyle().Foreground(corCinza)
	styleNomeChat   = lipgloss.NewStyle().Bold(true).Foreground(corCiano)
	styleNomePub    = lipgloss.NewStyle().Bold(true).Foreground(corAmarelo)
	styleNomeEnvio  = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	styleSistema    = lipgloss.NewStyle().Foreground(corAmarelo)
	styleErroTUI    = lipgloss.NewStyle().Foreground(corVermelho)
	styleConfirmTUI = lipgloss.NewStyle().Foreground(corVerde)
	styleInputPfx   = lipgloss.NewStyle().Foreground(corAzul).Bold(true)
	styleLog        = lipgloss.NewStyle().Foreground(lipgloss.Color("238"))
	styleLogSep     = lipgloss.NewStyle().Foreground(lipgloss.Color("236"))
	styleHint       = lipgloss.NewStyle().Foreground(lipgloss.Color("237"))
	styleColHeader  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("252")).Background(lipgloss.Color("234"))
)

// --- Modelo bubbletea ---

type tuiModel struct {
	viewport  viewport.Model
	input     textinput.Model
	cli       *CLI
	mensagens []string
	connCount int
	width     int
	height    int
	ready     bool

	// histórico de comandos (↑↓)
	historico  []string
	posHist    int    // -1 = não está navegando no histórico
	inputSalvo string // texto que estava no campo antes de pressionar ↑

	// tab-completion
	completions []string // candidatos gerados no primeiro Tab
	compIdx     int      // índice atual dentro de completions
	compBase    string   // texto original antes de entrar em modo completion

	// painel de log + comandos (toggle com /log live → 3 colunas)
	logVisivel  bool
	logViewport viewport.Model
	logLinhas   []string
	cmdViewport viewport.Model
	cmdLinhas   []string
}

type tickMsg struct{}

func tickCmd() tea.Cmd {
	return tea.Tick(2*time.Second, func(time.Time) tea.Msg { return tickMsg{} })
}

// atualizarAlturas recalcula a altura dos viewports de acordo com o tamanho
// do terminal e se o painel de log está visível ou não.
func (m *tuiModel) atualizarAlturas() {
	m.viewport.Width = m.width
	m.logViewport.Width = m.width
	if m.logVisivel {
		// 3 colunas: status(1)+sep(1)+header(1)+blank(1)+sep(1)+input(1) = 6 fixas
		vpH := m.height - 6
		if vpH < 1 {
			vpH = 1
		}
		colW := (m.width - 2) / 3
		extra := m.width - 2 - colW*3
		m.cmdViewport.Width = colW
		m.cmdViewport.Height = vpH
		m.viewport.Width = colW + extra
		m.viewport.Height = vpH
		m.logViewport.Width = colW
		m.logViewport.Height = vpH
	} else {
		// 1 coluna: status(1)+sep(1)+sep(1)+input(1) = 4 fixas
		vpH := m.height - 4
		if vpH < 1 {
			vpH = 1
		}
		m.viewport.Width = m.width
		m.viewport.Height = vpH
		m.cmdViewport.Width = m.width
		m.cmdViewport.Height = vpH
	}
}

// iniciarTUI cria o programa bubbletea, registra-o no pacote ui e o executa.
func iniciarTUI(c *CLI) error {
	// Logs vão para arquivo + painel interno do TUI; remove escrita em stderr.
	registro.SilenciarTerminal(ui.LogWriter())
	m := novaTUIModel(c)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	ui.DefinirPrograma(p)
	_, err := p.Run()
	return err
}

func novaTUIModel(c *CLI) tuiModel {
	ti := textinput.New()
	ti.Placeholder = "/help para listar comandos · ↑↓ histórico · Tab autocompleta · PgUp/PgDn rola"
	ti.Prompt = "" // evita o "> " padrão que duplicaria nosso prefixo no View
	ti.Focus()
	ti.CharLimit = 512

	return tuiModel{
		cli:        c,
		input:      ti,
		posHist:    -1,
		logVisivel: true, // inicia já com as 3 colunas
	}
}

func (m tuiModel) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, tickCmd())
}

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmdVP, cmdInput tea.Cmd

	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if !m.ready {
			m.viewport = viewport.New(m.width, 1)
			m.logViewport = viewport.New(m.width, 1)
			m.cmdViewport = viewport.New(m.width, 1)
			m.viewport.SetContent(strings.Join(m.mensagens, "\n"))
			m.logViewport.SetContent(strings.Join(m.logLinhas, "\n"))
			m.cmdViewport.SetContent(strings.Join(m.cmdLinhas, "\n"))
			m.viewport.GotoBottom()
			m.logViewport.GotoBottom()
			m.cmdViewport.GotoBottom()
			m.ready = true
		}
		m.atualizarAlturas()
		m.input.Width = m.width - 3

	case tickMsg:
		m.connCount = len(m.cli.gerenciador.Listar())
		cmds = append(cmds, tickCmd())

	case ui.MsgUI:
		switch msg.Tipo {
		case "quit":
			return m, tea.Sequence(
				func() tea.Msg { m.cli.cleanup(); return nil },
				tea.Quit,
			)
		case "log_toggle":
			m.logVisivel = !m.logVisivel
			m.atualizarAlturas()
			if m.logVisivel {
				m.logViewport.SetContent(strings.Join(m.logLinhas, "\n"))
				m.logViewport.GotoBottom()
				m.cmdViewport.SetContent(strings.Join(m.cmdLinhas, "\n"))
				m.cmdViewport.GotoBottom()
			}
		default:
			linha := formatarLinha(msg)
			if linha == "" {
				break
			}
			switch {
			case msg.Tipo == "log" || msg.Tipo == "erro" || msg.Tipo == "sistema":
				// Erros, sistema e logs internos → coluna Log (direita).
				naBase := m.logViewport.AtBottom()
				m.logLinhas = append(m.logLinhas, linha)
				m.logViewport.SetContent(strings.Join(m.logLinhas, "\n"))
				if naBase {
					m.logViewport.GotoBottom()
				}
			case isTipoComando(msg.Tipo):
				// Saída de comandos → coluna Comandos (esquerda).
				naBase := m.cmdViewport.AtBottom()
				m.cmdLinhas = append(m.cmdLinhas, linha)
				m.cmdViewport.SetContent(strings.Join(m.cmdLinhas, "\n"))
				if naBase {
					m.cmdViewport.GotoBottom()
				}
			default:
				// Mensagens de chat → coluna Mensagens (meio).
				naBase := m.viewport.AtBottom()
				m.mensagens = append(m.mensagens, linha)
				m.viewport.SetContent(strings.Join(m.mensagens, "\n"))
				if naBase {
					m.viewport.GotoBottom()
				}
			}
		}

	case tea.KeyMsg:
		// Qualquer tecla que não seja Tab encerra o modo completion.
		if msg.Type != tea.KeyTab {
			m.completions = nil
		}

		switch msg.Type {

		case tea.KeyCtrlC:
			return m, tea.Sequence(
				func() tea.Msg { m.cli.cleanup(); return nil },
				tea.Quit,
			)

		// Histórico: ↑ vai para o comando anterior
		case tea.KeyUp:
			if len(m.historico) == 0 {
				return m, nil
			}
			if m.posHist == -1 {
				m.inputSalvo = m.input.Value()
				m.posHist = len(m.historico) - 1
			} else if m.posHist > 0 {
				m.posHist--
			}
			m.input.SetValue(m.historico[m.posHist])
			return m, nil

		// Histórico: ↓ avança para o comando mais recente
		case tea.KeyDown:
			if m.posHist == -1 {
				return m, nil
			}
			if m.posHist < len(m.historico)-1 {
				m.posHist++
				m.input.SetValue(m.historico[m.posHist])
			} else {
				m.posHist = -1
				m.input.SetValue(m.inputSalvo)
			}
			return m, nil

		// TAB completion: cicla entre candidatos
		case tea.KeyTab:
			if m.completions == nil {
				// Gera candidatos na primeira vez
				m.compBase = m.input.Value()
				m.completions = gerarCompletions(m.compBase, m.cli)
				if len(m.completions) == 0 {
					m.completions = nil
					return m, nil
				}
				m.compIdx = 0
			} else {
				// Cicla pelo próximo candidato; passado o último, volta ao original
				m.compIdx = (m.compIdx + 1) % (len(m.completions) + 1)
			}
			if m.compIdx < len(m.completions) {
				m.input.SetValue(m.completions[m.compIdx])
			} else {
				m.input.SetValue(m.compBase)
			}
			return m, nil

		case tea.KeyPgUp:
			m.viewport.HalfViewUp()
			return m, nil

		case tea.KeyPgDown:
			m.viewport.HalfViewDown()
			return m, nil

		case tea.KeyEnter:
			texto := strings.TrimSpace(m.input.Value())
			m.input.SetValue("")
			m.posHist = -1
			m.inputSalvo = ""
			if texto == "" {
				break
			}
			// Adiciona ao histórico (sem duplicatas consecutivas)
			if len(m.historico) == 0 || m.historico[len(m.historico)-1] != texto {
				m.historico = append(m.historico, texto)
			}
			// Processa em goroutine para não bloquear o render
			cmd := tea.Cmd(func() tea.Msg {
				m.cli.processar(texto)
				return nil
			})
			cmds = append(cmds, cmd)
		}
	}

	var cmdLog, cmdCmd tea.Cmd
	m.viewport, cmdVP = m.viewport.Update(msg)
	m.logViewport, cmdLog = m.logViewport.Update(msg)
	m.cmdViewport, cmdCmd = m.cmdViewport.Update(msg)
	m.input, cmdInput = m.input.Update(msg)
	cmds = append(cmds, cmdVP, cmdLog, cmdCmd, cmdInput)
	return m, tea.Batch(cmds...)
}

func (m tuiModel) View() string {
	if !m.ready {
		return "Inicializando..."
	}

	// Barra de status
	esq := fmt.Sprintf("P2P Chat — %s — porta %d", m.cli.cfg.MeuID(), m.cli.cfg.Porta)
	dir := fmt.Sprintf("%d conectado(s)", m.connCount)
	espaco := m.width - 2 - lipgloss.Width(esq) - lipgloss.Width(dir)
	if espaco < 1 {
		espaco = 1
	}
	statusBar := styleStatusBar.Width(m.width).Render(
		esq + strings.Repeat(" ", espaco) + dir,
	)

	sep := styleSep.Render(strings.Repeat("─", m.width))
	inputLinha := styleInputPfx.Render(">") + " " + m.input.View()

	if m.logVisivel {
		div := styleLogSep.Render("│")

		// Linha de títulos das colunas.
		tituloCol := func(texto string, largura int) string {
			return styleColHeader.Width(largura).Render(" " + texto)
		}
		headerRow := lipgloss.JoinHorizontal(lipgloss.Top,
			tituloCol("Comandos", m.cmdViewport.Width),
			div,
			tituloCol("Mensagens", m.viewport.Width),
			div,
			tituloCol("Log", m.logViewport.Width),
		)

		// Divisor vertical entre colunas (altura do viewport).
		h := m.viewport.Height
		divV := styleLogSep.Render(strings.Repeat("│\n", h-1) + "│")

		colunas := lipgloss.JoinHorizontal(lipgloss.Top,
			m.cmdViewport.View(),
			divV,
			m.viewport.View(),
			divV,
			m.logViewport.View(),
		)
		return lipgloss.JoinVertical(lipgloss.Left,
			statusBar,
			sep,
			headerRow,
			"",
			colunas,
			sep,
			inputLinha,
		)
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		statusBar,
		sep,
		m.viewport.View(),
		sep,
		inputLinha,
	)
}

// formatarLinha converte um MsgUI num string estilizado para o viewport.
// Mensagens de chat e eventos de sistema recebem timestamp [HH:MM].
// Saída de comandos (/peers, /help etc.) é exibida sem timestamp para não
// poluir blocos multi-linha.
func formatarLinha(msg ui.MsgUI) string {
	hora := styleTimestamp.Render(time.Now().Format("15:04"))

	switch msg.Tipo {
	case "send":
		num := ui.RegistrarPeer(msg.De)
		remetente := styleNomeChat.Render(fmt.Sprintf("[%d] %s", num, msg.De))
		return fmt.Sprintf("%s %s: %s", hora, remetente, msg.Conteudo)

	case "pub":
		num := ui.RegistrarPeer(msg.De)
		tag := styleSistema.Render(fmt.Sprintf("[PUB %s]", msg.Destino))
		remetente := styleNomePub.Render(fmt.Sprintf("[%d] %s", num, msg.De))
		return fmt.Sprintf("%s %s %s: %s", hora, tag, remetente, msg.Conteudo)

	case "enviado":
		num := ui.RegistrarPeer(msg.Destino)
		destino := styleNomeEnvio.Render(fmt.Sprintf("[%d] %s", num, msg.Destino))
		return fmt.Sprintf("%s %s %s: %s",
			hora, styleNomeEnvio.Render("você →"), destino, msg.Conteudo)

	case "sistema":
		return fmt.Sprintf("%s %s %s",
			hora, styleSistema.Render("*"), styleSistema.Render(msg.Conteudo))

	case "confirmacao":
		return fmt.Sprintf("%s %s", hora, styleConfirmTUI.Render("✓ "+msg.Conteudo))

	case "erro":
		return fmt.Sprintf("%s %s", hora, styleErroTUI.Render("ERRO: "+msg.Conteudo))

	case "log":
		return styleLog.Render(msg.Conteudo)

	case "info":
		// Saída de comandos (/peers, /help, /conn…): sem timestamp.
		// O conteúdo já pode conter códigos ANSI embutidos — não aplicar
		// lipgloss por cima para não quebrar os resets \033[0m internos.
		return msg.Conteudo

	case "cmd_sep":
		// Separador ocupa toda a largura da coluna (viewport trunca o excesso).
		sep := "─ " + msg.Conteudo + " " + strings.Repeat("─", 200)
		return styleLogSep.Render(sep)

	default:
		return ""
	}
}

// isTipoComando retorna true para mensagens que são saída de comandos
// (/peers, /conn, /help, confirmações) — vão para a coluna Comandos.
func isTipoComando(tipo string) bool {
	switch tipo {
	case "info", "confirmacao", "cmd_sep":
		return true
	}
	return false
}

// gerarCompletions retorna os candidatos de completion para o texto atual.
func gerarCompletions(linha string, c *CLI) []string {
	partes := strings.Fields(linha)
	var comp []string

	// Completa nomes de comandos enquanto o usuário ainda está digitando o nome
	// (começa com "/" e não tem espaço após o comando).
	if strings.HasPrefix(linha, "/") && len(partes) <= 1 && !strings.HasSuffix(linha, " ") {
		prefix := strings.TrimSpace(linha)
		for _, cmd := range comandos {
			if strings.HasPrefix(cmd, prefix) {
				comp = append(comp, cmd+" ")
			}
		}
		return comp
	}

	// Completa IDs de peer após "/msg "
	if len(partes) >= 1 && partes[0] == "/msg" && len(partes) <= 2 {
		prefix := ""
		if len(partes) == 2 {
			prefix = partes[1]
		}
		for _, id := range ui.ListarIDsPeers() {
			if strings.HasPrefix(id, prefix) {
				comp = append(comp, "/msg "+id+" ")
			}
		}
	}

	return comp
}
