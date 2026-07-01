// Grupo 10 — Redes de Computadores
// Fabio Willian Alves Silva, 251020487
// Gustavo Vieira de Araujo, 211068440
// Joao Francisco de Sousa Torres, 251037072

package rede

import (
	"encoding/json"
	"time"

	"cliente-p2p/configuracao"
	"cliente-p2p/peer"
	"cliente-p2p/protocolo"
	"cliente-p2p/registro"
	"cliente-p2p/roteador"
	"cliente-p2p/ui"
)

// iniciarLeitor lê continuamente mensagens de conexao e as despacha para o
// handler correto, de acordo com o campo "type". Roda em goroutine própria;
// ao sair (erro de leitura, BYE ou BYE_OK), remove a conexão do
// GerenciadorConexoes e a fecha.
func iniciarLeitor(conexao *peer.ConexaoPeer, gerenciador *peer.GerenciadorConexoes, tabela *peer.TabelaPeers, rot *roteador.Roteador, cfg *configuracao.Configuracao) {
	encerramentoLimpo := false
	defer func() {
		gerenciador.Remover(conexao.IDPeer)
		conexao.Fechar()
		registro.Depurar("Leitor", "conexão com %s encerrada", conexao.IDPeer)
		if !encerramentoLimpo {
			num := ui.RegistrarPeer(conexao.IDPeer)
			ui.ImprimirSistema("[%d] %s desconectou", num, conexao.IDPeer)
		}
	}()

	meuID := cfg.MeuID()

	for {
		linha, err := conexao.LerLinha()
		if err != nil {
			registro.Depurar("Leitor", "erro de leitura de %s: %v", conexao.IDPeer, err)
			return
		}

		var base protocolo.TipoMensagem
		if err := json.Unmarshal(linha, &base); err != nil {
			registro.Alertar("Leitor", "JSON inválido de %s", conexao.IDPeer)
			continue
		}

		switch base.Tipo {
		case "PING":
			var ping protocolo.MensagemPing
			if err := json.Unmarshal(linha, &ping); err != nil {
				continue
			}
			pong := protocolo.MensagemPong{
				Tipo:      "PONG",
				IDMsg:     ping.IDMsg,
				Timestamp: ping.Timestamp,
				TTL:       1,
			}
			if err := conexao.EscreverJSON(pong); err != nil {
				return
			}
			registro.Depurar("Leitor", "PING de %s -> PONG enviado", conexao.IDPeer)

		case "PONG":
			var pong protocolo.MensagemPong
			if err := json.Unmarshal(linha, &pong); err != nil {
				continue
			}
			select {
			case conexao.CanalPong <- pong:
			default:
			}

		case "SEND":
			var msg protocolo.MensagemEnvio
			if err := json.Unmarshal(linha, &msg); err != nil {
				continue
			}
			rot.TratarMensagemRecebida(conexao, msg)

		case "PUB":
			var msg protocolo.MensagemPublicacao
			if err := json.Unmarshal(linha, &msg); err != nil {
				continue
			}
			rot.TratarPublicacaoRecebida(msg)

		case "ACK":
			var msg protocolo.MensagemConfirmacao
			if err := json.Unmarshal(linha, &msg); err != nil {
				continue
			}
			conexao.ConfirmarRecebimento(msg.IDMsg)
			registro.Depurar("Leitor", "ACK de %s para msg %s", conexao.IDPeer, msg.IDMsg)

		case "BYE":
			var msg protocolo.MensagemTchau
			if err := json.Unmarshal(linha, &msg); err != nil {
				continue
			}
			tchauOk := protocolo.MensagemTchauOk{
				Tipo:    "BYE_OK",
				IDMsg:   msg.IDMsg,
				Origem:  meuID,
				Destino: conexao.IDPeer,
				TTL:     1,
			}
			_ = conexao.EscreverJSON(tchauOk)
			num := ui.RegistrarPeer(conexao.IDPeer)
			ui.ImprimirSistema("[%d] %s encerrou a sessão", num, conexao.IDPeer)
			// Evita que o Conector tente reconectar imediatamente a um peer
			// que acabou de avisar que está saindo; ele só volta a ATIVO
			// numa próxima descoberta (DISCOVER) que o confirme.
			tabela.MarcarObsoleto(conexao.IDPeer)
			encerramentoLimpo = true
			return

		case "BYE_OK":
			select {
			case conexao.CanalConfirmacaoBye <- struct{}{}:
			default:
			}
			encerramentoLimpo = true
			return

		default:
			registro.Alertar("Leitor", "tipo desconhecido de %s: %s", conexao.IDPeer, base.Tipo)
		}
	}
}

// EnviarTchau envia BYE para conexao e aguarda BYE_OK por até timeout antes
// de fechar a conexão. Usado no encerramento controlado (/quit e Ctrl+C).
func EnviarTchau(conexao *peer.ConexaoPeer, cfg *configuracao.Configuracao, timeout time.Duration) {
	tchau := protocolo.MensagemTchau{
		Tipo:    "BYE",
		IDMsg:   protocolo.GerarIDMensagem(),
		Origem:  cfg.MeuID(),
		Destino: conexao.IDPeer,
		Motivo:  "encerrando sessão",
		TTL:     1,
	}
	_ = conexao.EscreverJSON(tchau)

	select {
	case <-conexao.CanalConfirmacaoBye:
		registro.Informar("BYE", "BYE_OK recebido de %s", conexao.IDPeer)
	case <-time.After(timeout):
		registro.Alertar("BYE", "timeout aguardando BYE_OK de %s", conexao.IDPeer)
	}
	conexao.Fechar()
}
