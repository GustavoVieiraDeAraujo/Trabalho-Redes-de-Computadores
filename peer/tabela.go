package peer

import (
	"sync"

	"cliente-p2p/protocolo"
)

// TabelaPeers mantém o conjunto de peers conhecidos (descobertos via
// Rendezvous) e o estado de cada um (ATIVO/OBSOLETO). É segura para uso
// concorrente.
type TabelaPeers struct {
	mutex  sync.RWMutex
	tabela map[string]protocolo.RegistroPeer
}

// NovaTabelaPeers cria uma TabelaPeers vazia.
func NovaTabelaPeers() *TabelaPeers {
	return &TabelaPeers{tabela: make(map[string]protocolo.RegistroPeer)}
}

// Atualizar insere ou atualiza o registro de um peer. Peers novos entram
// como ATIVO.
func (t *TabelaPeers) Atualizar(peer protocolo.RegistroPeer) {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	if _, existe := t.tabela[peer.Identificador()]; !existe {
		peer.Estado = protocolo.EstadoAtivo
	}
	t.tabela[peer.Identificador()] = peer
}

// MarcarTodosObsoletos marca todos os peers como OBSOLETO. Usado antes de
// processar uma nova resposta de DISCOVER, para detectar peers que saíram.
func (t *TabelaPeers) MarcarTodosObsoletos() {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	for id, p := range t.tabela {
		p.Estado = protocolo.EstadoObsoleto
		t.tabela[id] = p
	}
}

// MarcarAtivo marca um peer específico como ATIVO.
func (t *TabelaPeers) MarcarAtivo(idPeer string) {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	if p, ok := t.tabela[idPeer]; ok {
		p.Estado = protocolo.EstadoAtivo
		t.tabela[idPeer] = p
	}
}

// Listar retorna uma cópia de todos os peers conhecidos.
func (t *TabelaPeers) Listar() []protocolo.RegistroPeer {
	t.mutex.RLock()
	defer t.mutex.RUnlock()
	saida := make([]protocolo.RegistroPeer, 0, len(t.tabela))
	for _, p := range t.tabela {
		saida = append(saida, p)
	}
	return saida
}

// Obter retorna o registro de um peer pelo seu identificador.
func (t *TabelaPeers) Obter(idPeer string) (protocolo.RegistroPeer, bool) {
	t.mutex.RLock()
	defer t.mutex.RUnlock()
	p, ok := t.tabela[idPeer]
	return p, ok
}

// Remover apaga o registro de um peer da tabela.
func (t *TabelaPeers) Remover(idPeer string) {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	delete(t.tabela, idPeer)
}
