// Grupo 10 — Redes de Computadores
// Fabio Willian Alves Silva, 251020487
// Gustavo Vieira de Araujo, 211068440
// Joao Francisco de Sousa Torres, 251037072

// Comando cliente-p2p é o ponto de entrada do cliente de chat P2P.
//
// Fluxo de inicialização: carrega config.json, registra o peer no servidor
// Rendezvous, inicia o servidor TCP para conexões entrantes, a descoberta
// periódica de peers (DISCOVER) e o conector de saída (HELLO/HELLO_OK).
// Por fim, entrega o controle à interface de linha de comando.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"cliente-p2p/cli"
	"cliente-p2p/configuracao"
	"cliente-p2p/descoberta"
	"cliente-p2p/peer"
	"cliente-p2p/rede"
	"cliente-p2p/rendezvous"
	"cliente-p2p/roteador"
	"cliente-p2p/ui"
)

func main() {
	caminhoConfig := flag.String("config", "config.json", "caminho para o arquivo de configuração")
	flag.Parse()

	cfg, err := configuracao.CarregarConfiguracao(*caminhoConfig)
	if err != nil {
		fmt.Fprintln(os.Stderr, "erro ao carregar configuração:", err)
		os.Exit(1)
	}

	rdv := rendezvous.NovoClienteRendezvous(cfg)
	if err := registrarComRetry(rdv); err != nil {
		fmt.Fprintln(os.Stderr, "falha ao registrar no rendezvous:", err)
		os.Exit(1)
	}

	tabela := peer.NovaTabelaPeers()
	gerenciador := peer.NovoGerenciadorConexoes()
	rot := roteador.NovoRoteador(gerenciador, cfg)
	conector := rede.NovoConector(cfg, tabela, gerenciador, rot)

	servidor := rede.NovoServidor(cfg, gerenciador, rot)
	if err := servidor.Iniciar(); err != nil {
		fmt.Fprintln(os.Stderr, "falha ao iniciar servidor:", err)
		os.Exit(1)
	}

	go descoberta.IniciarLoopDescoberta(rdv, tabela, conector, cfg)
	go conector.Iniciar()
	go rdv.IniciarRenovacaoPeriodica()

	// Encerramento limpo via SIGINT/SIGTERM: sinaliza ao TUI para executar
	// o fluxo de encerramento (BYE + unregister) e restaurar o terminal.
	canalSinal := make(chan os.Signal, 1)
	signal.Notify(canalSinal, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-canalSinal
		ui.EnviarMsg(ui.MsgUI{Tipo: "quit"})
	}()

	cli.NovaCLI(cfg, tabela, gerenciador, rdv, rot, conector).Executar()
}

// registrarComRetry tenta REGISTER no Rendezvous até 5 vezes com backoff
// exponencial (1s, 2s, 4s, 8s, 16s) antes de desistir. Útil quando o
// servidor está temporariamente indisponível na inicialização.
func registrarComRetry(rdv *rendezvous.ClienteRendezvous) error {
	espera := time.Second
	for tentativa := 1; tentativa <= 5; tentativa++ {
		if err := rdv.Registrar(); err == nil {
			return nil
		} else if tentativa == 5 {
			return err
		} else {
			fmt.Fprintf(os.Stderr, "rendezvous indisponível, tentativa %d/5 — aguardando %s\n", tentativa, espera)
			time.Sleep(espera)
			espera *= 2
		}
	}
	return nil
}
