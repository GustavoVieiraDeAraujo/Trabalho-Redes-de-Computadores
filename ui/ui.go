// Grupo 10 — Redes de Computadores
// Fabio Willian Alves Silva, 251020487
// Gustavo Vieira de Araujo, 211068440
// Joao Francisco de Sousa Torres, 251037072

// Package ui centraliza a saída para o terminal e mantém o registro global
// de numeração de peers. Isso garante que o número exibido em /peers, /conn
// e nas mensagens recebidas seja sempre o mesmo, sem precisar rodar /peers
// antes de saber qual número usar no /msg.
package ui

import (
	"fmt"
	"sync"
	"time"
)

// Códigos ANSI de cor e formatação.
const (
	reset    = "\033[0m"
	bold     = "\033[1m"
	cinza    = "\033[90m"
	verde    = "\033[32m"
	ciano    = "\033[36m"
	amarelo  = "\033[33m"
	vermelho = "\033[31m"
)

// --- Registro global de numeração de peers ---

var (
	muNums    sync.RWMutex
	porID     = map[string]int{}
	porNumero = map[int]string{}
	proximo   = 1
)

// RegistrarPeer atribui (ou retorna o existente) número de atalho para id.
// Pode ser chamada de qualquer goroutine.
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

// --- Saída para o terminal ---

var (
	mu          sync.Mutex
	promptAtual = "> "
)

// DefinirPrompt atualiza a string do prompt que será reexibida após
// mensagens assíncronas.
func DefinirPrompt(p string) {
	mu.Lock()
	promptAtual = p
	mu.Unlock()
}

// imprimirAsync limpa a linha do terminal onde está o prompt/input do liner,
// imprime texto e reexibe o prompt. Deve ser chamada com mu adquirido.
func imprimirAsync(texto string) {
	fmt.Printf("\r\033[2K%s\n%s", texto, promptAtual)
}

// ImprimirMensagem exibe uma mensagem SEND recebida. Registra o peer
// automaticamente e mostra o número de atalho na frente do nome, para que
// o usuário saiba imediatamente qual número usar no /msg.
// Thread-safe — adequada para goroutines em background.
func ImprimirMensagem(de, conteudo string) {
	num := RegistrarPeer(de)
	mu.Lock()
	defer mu.Unlock()
	hora := time.Now().Format("15:04")
	imprimirAsync(fmt.Sprintf("%s[%s]%s %s[%d] %s%s: %s",
		cinza, hora, reset, bold+ciano, num, de, reset, conteudo))
}

// ImprimirPublicacao exibe uma mensagem PUB recebida, com número de atalho.
// Thread-safe — adequada para goroutines em background.
func ImprimirPublicacao(destino, de, conteudo string) {
	num := RegistrarPeer(de)
	mu.Lock()
	defer mu.Unlock()
	hora := time.Now().Format("15:04")
	imprimirAsync(fmt.Sprintf("%s[%s]%s %s[PUB %s]%s %s[%d] %s%s: %s",
		cinza, hora, reset, amarelo, destino, reset, bold, num, de, reset, conteudo))
}

// ImprimirSistema exibe uma notificação de sistema (conexão, BYE, ACK).
// Thread-safe — adequada para goroutines em background.
func ImprimirSistema(formato string, args ...interface{}) {
	mu.Lock()
	defer mu.Unlock()
	hora := time.Now().Format("15:04")
	msg := fmt.Sprintf(formato, args...)
	imprimirAsync(fmt.Sprintf("%s[%s]%s %s*%s %s", cinza, hora, reset, amarelo, reset, msg))
}

// Linha imprime uma linha de saída de comando (uso síncrono na goroutine
// principal, enquanto liner não está em modo raw).
func Linha(formato string, args ...interface{}) {
	fmt.Printf(formato+"\n", args...)
}

// Erro imprime uma mensagem de erro em vermelho (uso síncrono).
func Erro(formato string, args ...interface{}) {
	fmt.Printf("%sERRO: %s%s\n", vermelho, fmt.Sprintf(formato, args...), reset)
}

// Confirmacao imprime uma confirmação em verde (uso síncrono).
func Confirmacao(formato string, args ...interface{}) {
	fmt.Printf("%s✓ %s%s\n", verde, fmt.Sprintf(formato, args...), reset)
}
