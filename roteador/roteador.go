// Grupo 10 — Redes de Computadores
// Fabio Willian Alves Silva, 251020487
// Gustavo Vieira de Araujo, 211068440
// Joao Francisco de Sousa Torres, 251037072

// Package roteador implementa o envio de mensagens diretas (SEND/ACK) e de
// difusão (PUB), além do tratamento das mensagens SEND/PUB recebidas de
// outros peers.
package roteador

import (
	"fmt"
	"strings"
	"time"

	"cliente-p2p/configuracao"
	"cliente-p2p/peer"
	"cliente-p2p/protocolo"
	"cliente-p2p/registro"
	"cliente-p2p/ui"
)

// Roteador envia e recebe mensagens de aplicação (SEND, PUB) através das
// conexões mantidas pelo GerenciadorConexoes.
type Roteador struct {
	gerenciador *peer.GerenciadorConexoes
	cfg         *configuracao.Configuracao
}

// NovoRoteador cria um Roteador associado ao gerenciador de conexões e à
// configuração do peer local.
func NovoRoteador(gerenciador *peer.GerenciadorConexoes, cfg *configuracao.Configuracao) *Roteador {
	return &Roteador{gerenciador: gerenciador, cfg: cfg}
}

// Enviar envia uma mensagem direta (SEND) para idPeerDestino, exigindo
// confirmação (ACK). Aguarda o ACK por até 5 segundos em uma goroutine
// separada; se não chegar, registra um aviso no log.
func (r *Roteador) Enviar(idPeerDestino, conteudo string) error {
	conexao, ok := r.gerenciador.Obter(idPeerDestino)
	if !ok {
		return fmt.Errorf("sem conexão com %s", idPeerDestino)
	}

	idMsg := protocolo.GerarIDMensagem()
	msg := protocolo.MensagemEnvio{
		Tipo:              "SEND",
		IDMsg:             idMsg,
		Origem:            r.cfg.MeuID(),
		Destino:           idPeerDestino,
		Conteudo:          conteudo,
		RequerConfirmacao: true,
		TTL:               1,
	}

	canalAck := conexao.RegistrarEsperaConfirmacao(idMsg)
	if err := conexao.EscreverJSON(msg); err != nil {
		conexao.CancelarEsperaConfirmacao(idMsg)
		return err
	}
	registro.Informar("Roteador", "SEND -> %s: %s", idPeerDestino, conteudo)

	go func() {
		select {
		case <-canalAck:
			num := ui.RegistrarPeer(idPeerDestino)
			ui.Confirmacao("ACK [%d] %s", num, idPeerDestino)
		case <-time.After(5 * time.Second):
			registro.Alertar("Roteador", "timeout ACK de %s para msg %s", idPeerDestino, idMsg)
			conexao.CancelarEsperaConfirmacao(idMsg)
		}
	}()

	return nil
}

// Publicar envia uma mensagem de difusão (PUB) para todas as conexões
// ativas. destino pode ser "*" (todos) ou "#namespace" (apenas peers do
// namespace indicado).
func (r *Roteador) Publicar(destino, conteudo string) {
	meuID := r.cfg.MeuID()
	idMsg := protocolo.GerarIDMensagem()

	for _, conexao := range r.gerenciador.Listar() {
		if strings.HasPrefix(destino, "#") {
			namespace := destino[1:]
			partes := strings.SplitN(conexao.IDPeer, "@", 2)
			if len(partes) != 2 || partes[1] != namespace {
				continue
			}
		}

		msg := protocolo.MensagemPublicacao{
			Tipo:              "PUB",
			IDMsg:             idMsg,
			Origem:            meuID,
			Destino:           destino,
			Conteudo:          conteudo,
			RequerConfirmacao: false,
			TTL:               1,
		}
		if err := conexao.EscreverJSON(msg); err != nil {
			registro.Alertar("Roteador", "PUB falhou para %s: %v", conexao.IDPeer, err)
		}
	}
}

// TratarMensagemRecebida exibe uma mensagem SEND recebida e responde com
// ACK caso RequerConfirmacao seja verdadeiro.
func (r *Roteador) TratarMensagemRecebida(conexao *peer.ConexaoPeer, msg protocolo.MensagemEnvio) {
	ui.ImprimirMensagem(msg.Origem, msg.Conteudo)

	if msg.RequerConfirmacao {
		ack := protocolo.MensagemConfirmacao{
			Tipo:      "ACK",
			IDMsg:     msg.IDMsg,
			Timestamp: time.Now().UTC(),
			TTL:       1,
		}
		_ = conexao.EscreverJSON(ack)
	}
}

// TratarPublicacaoRecebida exibe uma mensagem PUB recebida de outro peer.
func (r *Roteador) TratarPublicacaoRecebida(msg protocolo.MensagemPublicacao) {
	ui.ImprimirPublicacao(msg.Destino, msg.Origem, msg.Conteudo)
}
