// Package protocolo define as estruturas de dados trocadas com o servidor
// Rendezvous e entre os peers. Os nomes dos campos Go foram traduzidos para
// português, mas as tags `json` permanecem em inglês porque fazem parte do
// protocolo de rede definido na especificação do trabalho — alterá-las
// quebraria a compatibilidade com o servidor Rendezvous e com outros peers.
package protocolo

import (
	"crypto/rand"
	"fmt"
	"time"
)

// Estados possíveis de um peer na tabela local.
const (
	EstadoAtivo    = "ATIVO"
	EstadoObsoleto = "OBSOLETO"
)

// RegistroPeer representa um peer retornado pelo comando DISCOVER do
// servidor Rendezvous.
type RegistroPeer struct {
	IP        string `json:"ip"`
	Porta     int    `json:"port"`
	Nome      string `json:"name"`
	Namespace string `json:"namespace"`
	TTL       int    `json:"ttl"`
	ExpiraEm  int    `json:"expires_in"`
	Estado    string `json:"-"`
}

// Identificador retorna o identificador completo do peer no formato
// "nome@namespace".
func (p RegistroPeer) Identificador() string {
	return p.Nome + "@" + p.Namespace
}

// RespostaRendezvous representa a resposta genérica do servidor Rendezvous
// para os comandos REGISTER, DISCOVER e UNREGISTER.
type RespostaRendezvous struct {
	Status   string         `json:"status"`
	TTL      int            `json:"ttl,omitempty"`
	IP       string         `json:"ip,omitempty"`
	Porta    int            `json:"port,omitempty"`
	Peers    []RegistroPeer `json:"peers,omitempty"`
	Mensagem string         `json:"message,omitempty"`
}

// TipoMensagem é usada para identificar o campo "type" de uma mensagem P2P
// antes de decodificá-la para a struct específica.
type TipoMensagem struct {
	Tipo string `json:"type"`
}

// MensagemHello é enviada ao estabelecer uma nova conexão P2P (handshake).
type MensagemHello struct {
	Tipo     string   `json:"type"`
	IDPeer   string   `json:"peer_id"`
	Versao   string   `json:"version"`
	Recursos []string `json:"features"`
	TTL      int      `json:"ttl"`
}

// MensagemPing é enviada periodicamente para manter a conexão viva
// (keep-alive) e medir o RTT.
type MensagemPing struct {
	Tipo      string    `json:"type"`
	IDMsg     string    `json:"msg_id"`
	Timestamp time.Time `json:"timestamp"`
	TTL       int       `json:"ttl"`
}

// MensagemPong é a resposta a uma MensagemPing.
type MensagemPong struct {
	Tipo      string    `json:"type"`
	IDMsg     string    `json:"msg_id"`
	Timestamp time.Time `json:"timestamp"`
	TTL       int       `json:"ttl"`
}

// MensagemEnvio representa uma mensagem direta (unicast) entre dois peers.
type MensagemEnvio struct {
	Tipo              string `json:"type"`
	IDMsg             string `json:"msg_id"`
	Origem            string `json:"src"`
	Destino           string `json:"dst"`
	Conteudo          string `json:"payload"`
	RequerConfirmacao bool   `json:"require_ack"`
	TTL               int    `json:"ttl"`
}

// MensagemConfirmacao confirma o recebimento de uma MensagemEnvio (ACK).
type MensagemConfirmacao struct {
	Tipo      string    `json:"type"`
	IDMsg     string    `json:"msg_id"`
	Timestamp time.Time `json:"timestamp"`
	TTL       int       `json:"ttl"`
}

// MensagemPublicacao representa uma mensagem de broadcast/namespace-cast
// (PUB), enviada para "*" (todos) ou "#namespace".
type MensagemPublicacao struct {
	Tipo              string `json:"type"`
	IDMsg             string `json:"msg_id"`
	Origem            string `json:"src"`
	Destino           string `json:"dst"`
	Conteudo          string `json:"payload"`
	RequerConfirmacao bool   `json:"require_ack"`
	TTL               int    `json:"ttl"`
}

// MensagemTchau (BYE) inicia o encerramento controlado de uma conexão P2P.
type MensagemTchau struct {
	Tipo    string `json:"type"`
	IDMsg   string `json:"msg_id"`
	Origem  string `json:"src"`
	Destino string `json:"dst"`
	Motivo  string `json:"reason"`
	TTL     int    `json:"ttl"`
}

// MensagemTchauOk (BYE_OK) confirma o encerramento solicitado por MensagemTchau.
type MensagemTchauOk struct {
	Tipo    string `json:"type"`
	IDMsg   string `json:"msg_id"`
	Origem  string `json:"src"`
	Destino string `json:"dst"`
	TTL     int    `json:"ttl"`
}

// GerarIDMensagem cria um identificador único (UUID v4) usado nos campos
// "msg_id" das mensagens do protocolo.
func GerarIDMensagem() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
