// Grupo 10 — Redes de Computadores
// Fabio Willian Alves Silva, 251020487
// Gustavo Vieira de Araujo, 211068440
// Joao Francisco de Sousa Torres, 251037072

// Package rendezvous implementa o cliente do protocolo Rendezvous
// (REGISTER, DISCOVER, UNREGISTER). Cada comando abre uma conexão TCP
// nova, envia uma única linha JSON e lê uma única linha JSON de resposta,
// conforme a especificação do servidor.
//
// Os nomes dos campos enviados ("type", "namespace", "name", "port", "ttl")
// permanecem em inglês propositalmente: são definidos pelo protocolo do
// servidor Rendezvous e não podem ser traduzidos.
package rendezvous

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"time"

	"cliente-p2p/configuracao"
	"cliente-p2p/protocolo"
	"cliente-p2p/registro"
)

// ClienteRendezvous comunica-se com o servidor Rendezvous configurado em
// cfg.EnderecoRendezvous:cfg.PortaRendezvous.
type ClienteRendezvous struct {
	cfg *configuracao.Configuracao
}

// NovoClienteRendezvous cria um ClienteRendezvous a partir da configuração
// do peer local.
func NovoClienteRendezvous(cfg *configuracao.Configuracao) *ClienteRendezvous {
	return &ClienteRendezvous{cfg: cfg}
}

func (r *ClienteRendezvous) endereco() string {
	return fmt.Sprintf("%s:%d", r.cfg.EnderecoRendezvous, r.cfg.PortaRendezvous)
}

// enviarComando abre uma conexão TCP, envia msg como uma linha JSON e
// retorna a resposta decodificada.
func (r *ClienteRendezvous) enviarComando(msg interface{}) (*protocolo.RespostaRendezvous, error) {
	conexao, err := net.DialTimeout("tcp", r.endereco(), 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("conexão: %w", err)
	}
	defer conexao.Close()
	conexao.SetDeadline(time.Now().Add(10 * time.Second))

	dados, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}
	if _, err := fmt.Fprintf(conexao, "%s\n", dados); err != nil {
		return nil, fmt.Errorf("escrita: %w", err)
	}

	var resp protocolo.RespostaRendezvous
	if err := json.NewDecoder(bufio.NewReader(conexao)).Decode(&resp); err != nil {
		return nil, fmt.Errorf("decodificação: %w", err)
	}
	return &resp, nil
}

// Registrar envia REGISTER ao servidor Rendezvous, anunciando este peer
// (nome, namespace, porta e TTL) para que outros peers possam descobri-lo.
func (r *ClienteRendezvous) Registrar() error {
	resp, err := r.enviarComando(map[string]interface{}{
		"type":      "REGISTER",
		"namespace": r.cfg.Namespace,
		"name":      r.cfg.Nome,
		"port":      r.cfg.Porta,
		"ttl":       r.cfg.TTL,
	})
	if err != nil {
		return err
	}
	if resp.Status != "OK" {
		return fmt.Errorf("registro: %s", resp.Mensagem)
	}
	registro.Informar("Rendezvous", "registrado como %s (ip=%s port=%d ttl=%ds)",
		r.cfg.MeuID(), resp.IP, resp.Porta, resp.TTL)
	return nil
}

// IniciarRenovacaoPeriodica reenvia REGISTER periodicamente para renovar o
// TTL deste peer no servidor Rendezvous, evitando que o registro expire
// (e seja descartado pelo servidor) em sessões mais longas que cfg.TTL.
// O intervalo entre renovações é metade do TTL configurado (com variação
// aleatória de ±10%), respeitando um mínimo de 5 segundos. Em caso de falha,
// tenta novamente com backoff exponencial (5s, 10s, 20s, ... até 60s) antes
// de retomar o ciclo normal. Deve ser chamada em uma goroutine própria; roda
// indefinidamente.
func (r *ClienteRendezvous) IniciarRenovacaoPeriodica() {
	for {
		time.Sleep(intervaloRenovacao(r.cfg.TTL))

		backoff := 5 * time.Second
		for {
			if err := r.Registrar(); err == nil {
				break
			} else {
				registro.Alertar("Rendezvous", "renovação de registro falhou: %v", err)
			}
			time.Sleep(backoff)
			if backoff < 60*time.Second {
				backoff *= 2
				if backoff > 60*time.Second {
					backoff = 60 * time.Second
				}
			}
		}
	}
}

// intervaloRenovacao calcula o tempo de espera até a próxima renovação de
// registro: metade do TTL (ou ttl-60, o que for menor), com variação
// aleatória de ±10% para evitar que múltiplos peers renovem exatamente ao
// mesmo tempo.
func intervaloRenovacao(ttl int) time.Duration {
	base := float64(ttl) / 2
	if maxBase := float64(ttl - 60); maxBase > 0 && maxBase < base {
		base = maxBase
	}
	if base < 5 {
		base = 5
	}
	jitter := base * (rand.Float64()*0.2 - 0.1)
	return time.Duration((base + jitter) * float64(time.Second))
}

// Descobrir envia DISCOVER ao servidor Rendezvous e retorna os peers
// registrados em namespace (ou em todos os namespaces, se vazio).
func (r *ClienteRendezvous) Descobrir(namespace string) ([]protocolo.RegistroPeer, error) {
	msg := map[string]interface{}{"type": "DISCOVER"}
	if namespace != "" {
		msg["namespace"] = namespace
	}
	resp, err := r.enviarComando(msg)
	if err != nil {
		return nil, err
	}
	if resp.Status != "OK" {
		return nil, fmt.Errorf("descoberta: %s", resp.Mensagem)
	}
	return resp.Peers, nil
}

// Desregistrar envia UNREGISTER ao servidor Rendezvous, removendo o
// registro deste peer.
func (r *ClienteRendezvous) Desregistrar() error {
	resp, err := r.enviarComando(map[string]interface{}{
		"type":      "UNREGISTER",
		"namespace": r.cfg.Namespace,
		"name":      r.cfg.Nome,
		"port":      r.cfg.Porta,
	})
	if err != nil {
		return err
	}
	if resp.Status != "OK" {
		return fmt.Errorf("desregistro: %s", resp.Mensagem)
	}
	registro.Informar("Rendezvous", "desregistrado com sucesso")
	return nil
}
