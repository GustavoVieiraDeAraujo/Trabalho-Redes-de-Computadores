// Grupo 10 — Redes de Computadores
// Fabio Willian Alves Silva, 251020487
// Gustavo Vieira de Araujo, 211068440
// Joao Francisco de Sousa Torres, 251037072

// Package registro fornece um logger simples com níveis ajustáveis em
// tempo de execução (DEBUG, INFO, WARN, ERROR). A saída vai para stderr
// até SilenciarTerminal ser chamado, a partir do qual é roteada para os
// writers extras informados (ex: ui.LogWriter() para o painel do TUI).
package registro

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync/atomic"
)

const (
	nivelDebug int32 = iota
	nivelInfo
	nivelAlerta
	nivelErro
)

var nivelAtual int32 = nivelInfo

var registradores = []*log.Logger{
	log.New(os.Stderr, "[DEBUG] ", log.Ldate|log.Ltime),
	log.New(os.Stderr, "[INFO]  ", log.Ldate|log.Ltime),
	log.New(os.Stderr, "[WARN]  ", log.Ldate|log.Ltime),
	log.New(os.Stderr, "[ERROR] ", log.Ldate|log.Ltime),
}

// DefinirNivel altera o nível mínimo de log exibido (DEBUG, INFO, WARN, ERROR).
func DefinirNivel(nivel string) {
	switch strings.ToUpper(nivel) {
	case "DEBUG":
		atomic.StoreInt32(&nivelAtual, nivelDebug)
	case "INFO":
		atomic.StoreInt32(&nivelAtual, nivelInfo)
	case "WARN":
		atomic.StoreInt32(&nivelAtual, nivelAlerta)
	case "ERROR":
		atomic.StoreInt32(&nivelAtual, nivelErro)
	default:
		Alertar("Registro", "nível desconhecido: %s", nivel)
	}
}

func registrar(nivel int32, modulo, formato string, args ...interface{}) {
	if nivel < atomic.LoadInt32(&nivelAtual) {
		return
	}
	registradores[nivel].Printf("[%s] %s", modulo, fmt.Sprintf(formato, args...))
}

// Depurar registra uma mensagem de nível DEBUG.
func Depurar(modulo, formato string, args ...interface{}) {
	registrar(nivelDebug, modulo, formato, args...)
}

// Informar registra uma mensagem de nível INFO.
func Informar(modulo, formato string, args ...interface{}) {
	registrar(nivelInfo, modulo, formato, args...)
}

// Alertar registra uma mensagem de nível WARN.
func Alertar(modulo, formato string, args ...interface{}) {
	registrar(nivelAlerta, modulo, formato, args...)
}

// Erro registra uma mensagem de nível ERROR.
func Erro(modulo, formato string, args ...interface{}) {
	registrar(nivelErro, modulo, formato, args...)
}

// SilenciarTerminal redireciona todos os loggers para os writers informados,
// removendo a escrita em stderr. Deve ser chamada quando o TUI ocupa o terminal.
// Se nenhum writer for fornecido, descarta toda a saída.
func SilenciarTerminal(extra ...io.Writer) {
	var saida io.Writer
	switch len(extra) {
	case 0:
		saida = io.Discard
	case 1:
		saida = extra[0]
	default:
		saida = io.MultiWriter(extra...)
	}
	for _, l := range registradores {
		l.SetOutput(saida)
	}
}
