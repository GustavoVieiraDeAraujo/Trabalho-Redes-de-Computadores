// Grupo 10 — Redes de Computadores
// Fabio Willian Alves Silva, 251020487
// Gustavo Vieira de Araujo, 211068440
// Joao Francisco de Sousa Torres, 251037072

package peer

import (
	"sync"

	"cliente-p2p/registro"
)

// GerenciadorConexoes mantém o mapa de conexões P2P ativas, indexadas pelo
// identificador do peer (name@namespace). É seguro para uso concorrente.
type GerenciadorConexoes struct {
	mutex    sync.RWMutex
	conexoes map[string]*ConexaoPeer
}

// NovoGerenciadorConexoes cria um GerenciadorConexoes vazio.
func NovoGerenciadorConexoes() *GerenciadorConexoes {
	return &GerenciadorConexoes{conexoes: make(map[string]*ConexaoPeer)}
}

// Adicionar registra uma nova conexão ativa.
func (g *GerenciadorConexoes) Adicionar(c *ConexaoPeer) {
	g.mutex.Lock()
	defer g.mutex.Unlock()
	g.conexoes[c.IDPeer] = c
	registro.Informar("Gerenciador", "conexão adicionada: %s (%s)", c.IDPeer, c.Direcao)
}

// Remover apaga a conexão associada a idPeer, se existir.
func (g *GerenciadorConexoes) Remover(idPeer string) {
	g.mutex.Lock()
	defer g.mutex.Unlock()
	delete(g.conexoes, idPeer)
	registro.Informar("Gerenciador", "conexão removida: %s", idPeer)
}

// Obter retorna a conexão associada a idPeer, se existir.
func (g *GerenciadorConexoes) Obter(idPeer string) (*ConexaoPeer, bool) {
	g.mutex.RLock()
	defer g.mutex.RUnlock()
	c, ok := g.conexoes[idPeer]
	return c, ok
}

// Possui indica se já existe uma conexão ativa com idPeer.
func (g *GerenciadorConexoes) Possui(idPeer string) bool {
	g.mutex.RLock()
	defer g.mutex.RUnlock()
	_, ok := g.conexoes[idPeer]
	return ok
}

// Listar retorna todas as conexões ativas.
func (g *GerenciadorConexoes) Listar() []*ConexaoPeer {
	g.mutex.RLock()
	defer g.mutex.RUnlock()
	saida := make([]*ConexaoPeer, 0, len(g.conexoes))
	for _, c := range g.conexoes {
		saida = append(saida, c)
	}
	return saida
}
