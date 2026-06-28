package peer

import (
	"bufio"
	"encoding/json"
	"net"
	"sync"
	"time"

	"cliente-p2p/protocolo"
)

// Direções possíveis de uma conexão P2P.
const (
	DirecaoEntrada = "entrada" // conexão aceita pelo nosso servidor TCP
	DirecaoSaida   = "saida"   // conexão que nós iniciamos
)

// ConexaoPeer encapsula uma conexão TCP persistente com outro peer,
// incluindo escrita serializada (mutex), canais para PONG/BYE_OK e o
// controle de confirmações (ACK) pendentes.
type ConexaoPeer struct {
	IDPeer  string
	Direcao string

	conexao      net.Conn
	escritor     *json.Encoder
	leitor       *bufio.Reader
	mutexEscrita sync.Mutex

	// CanalPong recebe as mensagens PONG lidas pelo leitor e consumidas
	// pela goroutine de keep-alive.
	CanalPong chan protocolo.MensagemPong

	// CanalConfirmacaoBye é sinalizado quando um BYE_OK é recebido,
	// confirmando o encerramento solicitado por nós.
	CanalConfirmacaoBye chan struct{}

	mutexConfirmacao   sync.Mutex
	esperasConfirmacao map[string]chan struct{}

	mutexRTT sync.RWMutex
	rttMedio float64

	fecharUmaVez sync.Once
	// CanalEncerramento é fechado quando a conexão é finalizada, sinalizando
	// todas as goroutines associadas (leitor, keep-alive) para que parem.
	CanalEncerramento chan struct{}
}

// NovaConexaoPeer cria uma ConexaoPeer a partir de uma conexão TCP já
// estabelecida (aceita pelo servidor ou aberta pelo conector).
func NovaConexaoPeer(idPeer string, conexao net.Conn, direcao string) *ConexaoPeer {
	return &ConexaoPeer{
		IDPeer:              idPeer,
		Direcao:             direcao,
		conexao:             conexao,
		escritor:            json.NewEncoder(conexao),
		leitor:              bufio.NewReaderSize(conexao, 33*1024),
		CanalPong:           make(chan protocolo.MensagemPong, 4),
		CanalConfirmacaoBye: make(chan struct{}, 1),
		esperasConfirmacao:  make(map[string]chan struct{}),
		CanalEncerramento:   make(chan struct{}),
	}
}

// EscreverJSON serializa v como JSON e escreve no socket, protegido por
// mutex e com timeout de escrita de 5 segundos.
func (c *ConexaoPeer) EscreverJSON(v interface{}) error {
	c.mutexEscrita.Lock()
	defer c.mutexEscrita.Unlock()
	c.conexao.SetWriteDeadline(time.Now().Add(5 * time.Second))
	return c.escritor.Encode(v)
}

// LerLinha lê uma linha completa (até '\n') do socket.
func (c *ConexaoPeer) LerLinha() ([]byte, error) {
	return c.leitor.ReadBytes('\n')
}

// Fechar encerra o socket e sinaliza CanalEncerramento. É seguro chamar
// múltiplas vezes e de goroutines diferentes — o fechamento ocorre apenas
// uma vez (sync.Once).
func (c *ConexaoPeer) Fechar() {
	c.fecharUmaVez.Do(func() {
		close(c.CanalEncerramento)
		c.conexao.Close()
	})
}

// RegistrarEsperaConfirmacao cria e registra um canal que será sinalizado
// quando o ACK referente a idMsg chegar (ver ConfirmarRecebimento).
func (c *ConexaoPeer) RegistrarEsperaConfirmacao(idMsg string) chan struct{} {
	canal := make(chan struct{}, 1)
	c.mutexConfirmacao.Lock()
	c.esperasConfirmacao[idMsg] = canal
	c.mutexConfirmacao.Unlock()
	return canal
}

// ConfirmarRecebimento sinaliza o canal de espera associado a idMsg,
// caso exista (chamado ao receber uma MensagemConfirmacao/ACK).
func (c *ConexaoPeer) ConfirmarRecebimento(idMsg string) {
	c.mutexConfirmacao.Lock()
	canal, ok := c.esperasConfirmacao[idMsg]
	if ok {
		delete(c.esperasConfirmacao, idMsg)
	}
	c.mutexConfirmacao.Unlock()
	if ok {
		select {
		case canal <- struct{}{}:
		default:
		}
	}
}

// CancelarEsperaConfirmacao remove a espera de ACK associada a idMsg,
// usado quando o timeout de confirmação expira.
func (c *ConexaoPeer) CancelarEsperaConfirmacao(idMsg string) {
	c.mutexConfirmacao.Lock()
	delete(c.esperasConfirmacao, idMsg)
	c.mutexConfirmacao.Unlock()
}

// AtualizarRTT atualiza o RTT médio usando média móvel exponencial:
// rtt = 0.8 * rtt_anterior + 0.2 * rtt_novo.
func (c *ConexaoPeer) AtualizarRTT(rttMs float64) {
	c.mutexRTT.Lock()
	defer c.mutexRTT.Unlock()
	if c.rttMedio == 0 {
		c.rttMedio = rttMs
	} else {
		c.rttMedio = 0.8*c.rttMedio + 0.2*rttMs
	}
}

// ObterRTT retorna o RTT médio atual em milissegundos (0 se ainda não
// houver medições).
func (c *ConexaoPeer) ObterRTT() float64 {
	c.mutexRTT.RLock()
	defer c.mutexRTT.RUnlock()
	return c.rttMedio
}
