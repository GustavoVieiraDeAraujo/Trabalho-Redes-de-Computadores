// Grupo 10 — Redes de Computadores
// Fabio Willian Alves Silva, 251020487
// Gustavo Vieira de Araujo, 211068440
// Joao Francisco de Sousa Torres, 251037072

// Package ui centraliza a saída para o terminal, separando mensagens de
// chat (assíncronas, vindas de goroutines em background) da saída de
// comandos (síncrona, na goroutine principal). A separação evita que
// mensagens recebidas corrompam o prompt do liner.
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

var (
	mu          sync.Mutex
	promptAtual = "> "
)

// DefinirPrompt atualiza a string do prompt que será reexibida após
// mensagens assíncronas (deve ser chamada sempre que o prompt mudar).
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

// ImprimirMensagem exibe uma mensagem SEND recebida de outro peer.
// Thread-safe — adequada para goroutines em background.
func ImprimirMensagem(de, conteudo string) {
	mu.Lock()
	defer mu.Unlock()
	hora := time.Now().Format("15:04")
	imprimirAsync(fmt.Sprintf("%s[%s]%s %s%s%s: %s",
		cinza, hora, reset, bold+ciano, de, reset, conteudo))
}

// ImprimirPublicacao exibe uma mensagem PUB recebida.
// Thread-safe — adequada para goroutines em background.
func ImprimirPublicacao(destino, de, conteudo string) {
	mu.Lock()
	defer mu.Unlock()
	hora := time.Now().Format("15:04")
	imprimirAsync(fmt.Sprintf("%s[%s]%s %s[PUB %s]%s %s%s%s: %s",
		cinza, hora, reset, amarelo, destino, reset, bold, de, reset, conteudo))
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
