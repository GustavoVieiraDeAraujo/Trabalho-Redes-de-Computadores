// Grupo 10 — Redes de Computadores
// Fabio Willian Alves Silva, 251020487
// Gustavo Vieira de Araujo, 211068440
// Joao Francisco de Sousa Torres, 251037072

// Package configuracao carrega e valida o arquivo config.json com os
// parâmetros do peer local (identidade, porta, tempos de keep-alive,
// reconexão e endereço do servidor Rendezvous).
package configuracao

import (
	"encoding/json"
	"fmt"
	"os"
)

// Configuracao representa os parâmetros lidos do arquivo config.json.
type Configuracao struct {
	Nome                   string `json:"nome"`
	Namespace              string `json:"namespace"`
	Porta                  int    `json:"porta"`
	TTL                    int    `json:"ttl"`
	IntervaloPingSegundos  int    `json:"intervalo_ping_s"`
	MaxTentativasReconexao int    `json:"max_tentativas_reconexao"`
	EnderecoRendezvous     string `json:"endereco_rendezvous"`
	PortaRendezvous        int    `json:"porta_rendezvous"`
	ArquivoLog             string `json:"arquivo_log"`
}

// MeuID retorna a identidade completa do peer no formato "nome@namespace".
func (c *Configuracao) MeuID() string {
	return c.Nome + "@" + c.Namespace
}

// CarregarConfiguracao lê o arquivo JSON em caminho e preenche valores
// padrão para os campos opcionais que não forem informados.
func CarregarConfiguracao(caminho string) (*Configuracao, error) {
	arquivo, err := os.Open(caminho)
	if err != nil {
		return nil, err
	}
	defer arquivo.Close()

	var cfg Configuracao
	if err := json.NewDecoder(arquivo).Decode(&cfg); err != nil {
		return nil, err
	}

	if cfg.IntervaloPingSegundos == 0 {
		cfg.IntervaloPingSegundos = 30
	}
	if cfg.MaxTentativasReconexao == 0 {
		cfg.MaxTentativasReconexao = 5
	}
	if cfg.EnderecoRendezvous == "" {
		cfg.EnderecoRendezvous = "pyp2p.mfcaetano.cc"
	}
	if cfg.PortaRendezvous == 0 {
		cfg.PortaRendezvous = 8080
	}
	if cfg.TTL == 0 {
		cfg.TTL = 3600
	}

	if cfg.Nome == "" {
		return nil, fmt.Errorf("config: campo 'nome' é obrigatório")
	}
	if cfg.Namespace == "" {
		return nil, fmt.Errorf("config: campo 'namespace' é obrigatório")
	}
	if cfg.Porta < 1 || cfg.Porta > 65535 {
		return nil, fmt.Errorf("config: 'porta' deve estar entre 1 e 65535 (valor: %d)", cfg.Porta)
	}

	return &cfg, nil
}
