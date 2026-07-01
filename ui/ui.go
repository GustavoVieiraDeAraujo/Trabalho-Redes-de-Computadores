// Grupo 10 — Redes de Computadores
// Fabio Willian Alves Silva, 251020487
// Gustavo Vieira de Araujo, 211068440
// Joao Francisco de Sousa Torres, 251037072

// Package ui centraliza a saída para o terminal e mantém o registro global
// de numeração de peers. Quando o TUI (bubbletea) estiver ativo, todas as
// funções roteiam mensagens via programa.Send; caso contrário imprimem
// diretamente no terminal com códigos ANSI (útil para testes e depuração).
package ui

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// MsgUI é o tipo de mensagem enviado ao programa bubbletea para que o TUI
// exiba notificações de forma assíncrona e segura.
type MsgUI struct {
	Tipo     string // "send", "pub", "sistema", "info", "erro", "confirmacao", "enviado", "quit"
	De       string
	Destino  string
	Conteudo string
}

// --- Registro global de numeração de peers ---

var (
	muNums    sync.RWMutex
	porID     = map[string]int{}
	porNumero = map[int]string{}
	proximo   = 1
)

// RegistrarPeer atribui (ou retorna o existente) número de atalho para id.
func RegistrarPeer(id string) int {
	muNums.Lock()
	defer muNums.Unlock()
	if n, ok := porID[id]; ok {
		return n
	}
	n := proximo
	proximo++
	porID[id] = n
	porNumero[n] = id
	return n
}

// ResolverPeer converte um argumento (número inteiro ou ID direto) para o
// identificador completo do peer. Retorna false se o número não existir.
func ResolverPeer(arg string) (string, bool) {
	var n int
	if _, err := fmt.Sscan(arg, &n); err == nil {
		muNums.RLock()
		id, ok := porNumero[n]
		muNums.RUnlock()
		return id, ok
	}
	return arg, arg != ""
}

// ListarIDsPeers retorna todos os IDs conhecidos (para tab-completion).
func ListarIDsPeers() []string {
	muNums.RLock()
	defer muNums.RUnlock()
	ids := make([]string, 0, len(porID))
	for id := range porID {
		ids = append(ids, id)
	}
	return ids
}

// --- Roteamento de saída: TUI ou terminal direto ---

var (
	mu       sync.Mutex
	programa *tea.Program

	// prompt só é usado no caminho de fallback (sem bubbletea).
	promptAtual = "> "
)

// DefinirPrograma registra o programa bubbletea para receber mensagens.
// Deve ser chamada antes de Executar().
func DefinirPrograma(p *tea.Program) {
	mu.Lock()
	programa = p
	mu.Unlock()
}

// DefinirPrompt atualiza o prompt exibido no fallback ANSI.
func DefinirPrompt(p string) {
	mu.Lock()
	promptAtual = p
	mu.Unlock()
}

// logWriter implementa io.Writer roteando cada linha para o painel de log do TUI.
type logWriter struct{}

func (logWriter) Write(p []byte) (int, error) {
	linha := strings.TrimRight(string(p), "\n\r")
	if linha != "" {
		EnviarMsg(MsgUI{Tipo: "log", Conteudo: linha})
	}
	return len(p), nil
}

// LogWriter retorna um io.Writer que envia cada linha ao painel de log do TUI.
// Usado pelo pacote registro para rotear logs ao painel interno.
func LogWriter() io.Writer {
	return logWriter{}
}

// EnviarMsg envia msg ao TUI bubbletea se estiver ativo; caso contrário
// imprime diretamente no terminal com formatação ANSI.
func EnviarMsg(msg MsgUI) {
	mu.Lock()
	p := programa
	mu.Unlock()

	if p != nil {
		p.Send(msg)
		return
	}

	// --- Fallback terminal ANSI ---
	const (
		reset    = "\033[0m"
		bold     = "\033[1m"
		cinza    = "\033[90m"
		verde    = "\033[32m"
		ciano    = "\033[36m"
		amarelo  = "\033[33m"
		vermelho = "\033[31m"
	)

	hora := time.Now().Format("15:04")

	mu.Lock()
	defer mu.Unlock()

	pr := promptAtual
	flush := func(linha string) {
		fmt.Printf("\r\033[2K%s\n%s", linha, pr)
	}

	switch msg.Tipo {
	case "send":
		num := RegistrarPeer(msg.De)
		flush(fmt.Sprintf("%s[%s]%s %s[%d] %s%s: %s",
			cinza, hora, reset, bold+ciano, num, msg.De, reset, msg.Conteudo))
	case "pub":
		num := RegistrarPeer(msg.De)
		flush(fmt.Sprintf("%s[%s]%s %s[PUB %s]%s %s[%d] %s%s: %s",
			cinza, hora, reset, amarelo, msg.Destino, reset, bold, num, msg.De, reset, msg.Conteudo))
	case "sistema":
		flush(fmt.Sprintf("%s[%s]%s %s*%s %s", cinza, hora, reset, amarelo, reset, msg.Conteudo))
	case "confirmacao", "enviado":
		flush(fmt.Sprintf("%s[%s]%s %s✓%s %s", cinza, hora, reset, verde, reset, msg.Conteudo))
	case "erro":
		flush(fmt.Sprintf("%sERRO:%s %s", vermelho, reset, msg.Conteudo))
	default: // "info" e qualquer outro
		flush(msg.Conteudo)
	}
}

// --- Funções de alto nível usadas pelos outros pacotes ---

// ImprimirMensagem exibe uma mensagem SEND recebida com número de atalho.
func ImprimirMensagem(de, conteudo string) {
	EnviarMsg(MsgUI{Tipo: "send", De: de, Conteudo: conteudo})
}

// ImprimirPublicacao exibe uma mensagem PUB recebida com número de atalho.
func ImprimirPublicacao(destino, de, conteudo string) {
	EnviarMsg(MsgUI{Tipo: "pub", De: de, Destino: destino, Conteudo: conteudo})
}

// ImprimirSistema exibe uma notificação de sistema (conexão, BYE, ACK).
func ImprimirSistema(formato string, args ...interface{}) {
	EnviarMsg(MsgUI{Tipo: "sistema", Conteudo: fmt.Sprintf(formato, args...)})
}

// Linha imprime uma linha de saída de comando.
func Linha(formato string, args ...interface{}) {
	EnviarMsg(MsgUI{Tipo: "info", Conteudo: fmt.Sprintf(formato, args...)})
}

// Erro imprime uma mensagem de erro.
func Erro(formato string, args ...interface{}) {
	EnviarMsg(MsgUI{Tipo: "erro", Conteudo: fmt.Sprintf(formato, args...)})
}

// Confirmacao imprime uma confirmação de sucesso.
func Confirmacao(formato string, args ...interface{}) {
	EnviarMsg(MsgUI{Tipo: "confirmacao", Conteudo: fmt.Sprintf(formato, args...)})
}
