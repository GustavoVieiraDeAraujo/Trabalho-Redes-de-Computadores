// Grupo 10 — Redes de Computadores
// Fabio Willian Alves Silva, 251020487
// Gustavo Vieira de Araujo, 211068440
// Joao Francisco de Sousa Torres, 251037072

// Package registro fornece um logger simples com níveis ajustáveis em
// tempo de execução (DEBUG, INFO, WARN, ERROR), usado por todos os demais
// pacotes para registrar eventos com identificação do módulo de origem.
// Opcionalmente grava em arquivo (além do stderr) via IniciarLogArquivo.
package registro

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"
)

const (
	nivelDebug int32 = iota
	nivelInfo
	nivelAlerta
	nivelErro
)

var nivelAtual int32 = nivelInfo

var registradores = []*log.Logger{
	log.New(os.Stderr, "[DEBUG] ", log.Ltime),
	log.New(os.Stderr, "[INFO]  ", log.Ltime),
	log.New(os.Stderr, "[WARN]  ", log.Ltime),
	log.New(os.Stderr, "[ERROR] ", log.Ltime),
}

// IniciarLogArquivo cria um arquivo de log exclusivo para esta sessão,
// com timestamp no nome (ex: "cliente_20260621_103045.log"), e faz todos
// os loggers escreverem simultaneamente em stderr e no arquivo. Deve ser
// chamada uma única vez, durante a inicialização. Se caminho for vazio,
// não faz nada. Retorna o nome real do arquivo criado.
func IniciarLogArquivo(caminho string) (string, error) {
	if caminho == "" {
		return "", nil
	}
	ext := filepath.Ext(caminho)
	base := strings.TrimSuffix(caminho, ext)
	nomeReal := fmt.Sprintf("%s_%s%s", base, time.Now().Format("20060102_150405"), ext)

	arquivo, err := os.OpenFile(nomeReal, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return "", fmt.Errorf("não foi possível criar arquivo de log %q: %w", nomeReal, err)
	}
	saida := io.MultiWriter(os.Stderr, arquivo)
	prefixos := []string{"[DEBUG] ", "[INFO]  ", "[WARN]  ", "[ERROR] "}
	for i, l := range registradores {
		l.SetOutput(saida)
		l.SetPrefix(prefixos[i])
	}
	Informar("Registro", "log em arquivo iniciado: %s", nomeReal)
	return nomeReal, nil
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
