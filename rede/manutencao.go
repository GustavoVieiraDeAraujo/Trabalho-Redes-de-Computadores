// Grupo 10 — Redes de Computadores
// Fabio Willian Alves Silva, 251020487
// Gustavo Vieira de Araujo, 211068440
// Joao Francisco de Sousa Torres, 251037072

package rede

import (
	"time"

	"cliente-p2p/configuracao"
	"cliente-p2p/peer"
	"cliente-p2p/protocolo"
	"cliente-p2p/registro"
)

// iniciarManutencaoConexao envia PINGs periódicos (a cada
// cfg.IntervaloPingSegundos) e mede o RTT a partir do PONG correspondente.
// Se o PONG não chegar em 10 segundos, a conexão é fechada.
func iniciarManutencaoConexao(conexao *peer.ConexaoPeer, cfg *configuracao.Configuracao) {
	intervalo := time.Duration(cfg.IntervaloPingSegundos) * time.Second
	ticker := time.NewTicker(intervalo)
	defer ticker.Stop()

	for {
		select {
		case <-conexao.CanalEncerramento:
			return
		case <-ticker.C:
			idMsg := protocolo.GerarIDMensagem()
			ping := protocolo.MensagemPing{
				Tipo:      "PING",
				IDMsg:     idMsg,
				Timestamp: time.Now().UTC(),
				TTL:       1,
			}
			enviadoEm := time.Now()
			if err := conexao.EscreverJSON(ping); err != nil {
				registro.Alertar("Manutencao", "falha ao enviar PING para %s: %v", conexao.IDPeer, err)
				return
			}

			// Aguarda o PONG com msg_id correspondente.
			timeoutPong := time.NewTimer(10 * time.Second)
		aguardarPong:
			for {
				select {
				case <-conexao.CanalEncerramento:
					timeoutPong.Stop()
					return
				case pong := <-conexao.CanalPong:
					if pong.IDMsg == idMsg {
						rttMs := float64(time.Since(enviadoEm).Milliseconds())
						conexao.AtualizarRTT(rttMs)
						registro.Depurar("Manutencao", "PONG de %s, RTT=%.1fms", conexao.IDPeer, rttMs)
						timeoutPong.Stop()
						break aguardarPong
					}
				case <-timeoutPong.C:
					registro.Alertar("Manutencao", "timeout PONG de %s — fechando conexão", conexao.IDPeer)
					conexao.Fechar()
					return
				}
			}
		}
	}
}
