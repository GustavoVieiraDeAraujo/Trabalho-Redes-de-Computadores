// Grupo 10 — Redes de Computadores
// Fabio Willian Alves Silva, 251020487
// Gustavo Vieira de Araujo, 211068440
// Joao Francisco de Sousa Torres, 251037072

// Package descoberta executa periodicamente o comando DISCOVER no servidor
// Rendezvous e atualiza a TabelaPeers local com os peers encontrados.
package descoberta

import (
	"time"

	"cliente-p2p/configuracao"
	"cliente-p2p/peer"
	"cliente-p2p/protocolo"
	"cliente-p2p/rede"
	"cliente-p2p/registro"
	"cliente-p2p/rendezvous"
)

// IniciarLoopDescoberta executa uma descoberta imediatamente e depois a
// repete a cada 60 segundos, indefinidamente. Deve ser chamada em uma
// goroutine própria.
func IniciarLoopDescoberta(rdv *rendezvous.ClienteRendezvous, tabela *peer.TabelaPeers, conector *rede.Conector, cfg *configuracao.Configuracao) {
	executarDescoberta(rdv, tabela, conector, cfg)

	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		executarDescoberta(rdv, tabela, conector, cfg)
	}
}

// executarDescoberta envia DISCOVER, marca todos os peers conhecidos como
// OBSOLETO e atualiza/insere os peers retornados como ATIVO (exceto o
// próprio peer local). Em seguida dispara o Conector para reconciliar as
// conexões.
func executarDescoberta(rdv *rendezvous.ClienteRendezvous, tabela *peer.TabelaPeers, conector *rede.Conector, cfg *configuracao.Configuracao) {
	peers, err := rdv.Descobrir(cfg.Namespace)
	if err != nil {
		registro.Alertar("Descoberta", "falha: %v", err)
		return
	}

	tabela.MarcarTodosObsoletos()
	meuID := cfg.MeuID()
	contagem := 0
	for _, p := range peers {
		if p.Identificador() == meuID {
			continue
		}
		p.Estado = protocolo.EstadoAtivo
		tabela.Atualizar(p)
		contagem++
	}
	registro.Informar("Descoberta", "%d peer(s) encontrado(s) no namespace %s", contagem, cfg.Namespace)
	conector.Disparar()
}
