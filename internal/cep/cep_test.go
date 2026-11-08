package cep

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNormalize(t *testing.T) {
	type normalizeCase struct {
		name     string
		input    string
		expected string
		wantErr  bool
	}

	table := []normalizeCase{
		{"apenas dígitos", "01153000", "01153000", false},
		{"com hífen", "01153-000", "01153000", false},
		{"com ponto e hífen", "01.153-000", "01153000", false},
		{"curto demais", "0115300", "", true},
		{"longo demais", "011530001", "", true},
		{"com letra", "0115300a", "", true},
		{"vazio", "", "", true},
	}

	for _, item := range table {
		t.Run(item.name, func(t *testing.T) {
			result, err := Normalize(item.input)
			if item.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "cep inválido")
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, item.expected, result)
		})
	}
}

func TestFormat(t *testing.T) {
	assert.Equal(t, "01153-000", Format("01153000"))
	assert.Equal(t, "01153-000", Format("01153-000"))
}

func TestBrasilAPIFetch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/01153000" {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"message":"Todos os serviços de CEP retornaram erro."}`))
			return
		}
		w.Write([]byte(`{"cep":"01153000","state":"SP","city":"São Paulo",
			"neighborhood":"Barra Funda","street":"Rua Vitorino Carmilo","service":"correios"}`))
	}))
	defer server.Close()

	api := &BrasilAPI{BaseURL: server.URL, Client: server.Client()}

	t.Run("sucesso", func(t *testing.T) {
		result, err := api.Fetch(context.Background(), "01153000")
		assert.NoError(t, err)
		assert.Equal(t, Address{
			CEP:        "01153-000",
			Logradouro: "Rua Vitorino Carmilo",
			Bairro:     "Barra Funda",
			Cidade:     "São Paulo",
			UF:         "SP",
		}, result)
	})

	t.Run("404 vira ErrCEPNaoEncontrado", func(t *testing.T) {
		_, err := api.Fetch(context.Background(), "00000000")
		assert.ErrorIs(t, err, ErrCEPNaoEncontrado)
	})

	t.Run("nome do provedor", func(t *testing.T) {
		assert.Equal(t, "BrasilAPI", api.Name())
	})
}

func TestViaCEPFetch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// a ViaCEP responde 200 mesmo quando o CEP não existe
		if r.URL.Path != "/01153000/json/" {
			w.Write([]byte(`{"erro": "true"}`))
			return
		}
		w.Write([]byte(`{"cep":"01153-000","logradouro":"Rua Vitorino Carmilo","complemento":"lado ímpar",
			"bairro":"Barra Funda","localidade":"São Paulo","uf":"SP"}`))
	}))
	defer server.Close()

	api := &ViaCEP{BaseURL: server.URL, Client: server.Client()}

	t.Run("sucesso", func(t *testing.T) {
		result, err := api.Fetch(context.Background(), "01153000")
		assert.NoError(t, err)
		assert.Equal(t, Address{
			CEP:         "01153-000",
			Logradouro:  "Rua Vitorino Carmilo",
			Complemento: "lado ímpar",
			Bairro:      "Barra Funda",
			Cidade:      "São Paulo",
			UF:          "SP",
		}, result)
	})

	t.Run("flag erro vira ErrCEPNaoEncontrado", func(t *testing.T) {
		_, err := api.Fetch(context.Background(), "00000000")
		assert.ErrorIs(t, err, ErrCEPNaoEncontrado)
	})

	t.Run("nome do provedor", func(t *testing.T) {
		assert.Equal(t, "ViaCEP", api.Name())
	})
}

func TestIsErroFlag(t *testing.T) {
	assert.True(t, isErroFlag("true"))
	assert.True(t, isErroFlag("True"))
	assert.True(t, isErroFlag(true))
	assert.False(t, isErroFlag(false))
	assert.False(t, isErroFlag(nil))
	assert.False(t, isErroFlag(""))
}

// mock feito na mão porque além de controlar a latência de cada API ele precisa
// parar quando o contexto é cancelado, igual a uma requisição de verdade
type providerMock struct {
	name    string
	delay   time.Duration
	address Address
	err     error
}

func (p providerMock) Name() string { return p.name }

func (p providerMock) Fetch(ctx context.Context, _ string) (Address, error) {
	select {
	case <-time.After(p.delay):
		return p.address, p.err
	case <-ctx.Done():
		return Address{}, ctx.Err()
	}
}

func TestRaceVenceOMaisRapido(t *testing.T) {
	fast := providerMock{name: "Rapida", delay: 10 * time.Millisecond, address: Address{Cidade: "Curitiba"}}
	slow := providerMock{name: "Lenta", delay: 500 * time.Millisecond, address: Address{Cidade: "Sarandi"}}

	result, err := Race(context.Background(), "01153000", slow, fast)

	assert.NoError(t, err)
	assert.Equal(t, "Rapida", result.Provider)
	assert.Equal(t, "Curitiba", result.Address.Cidade, "a resposta da API lenta deveria ter sido descartada")
}

func TestRaceIgnoraProvedorQueFalha(t *testing.T) {
	broken := providerMock{name: "Quebrada", delay: time.Millisecond, err: errors.New("500 interno")}
	ok := providerMock{name: "Boa", delay: 30 * time.Millisecond, address: Address{Cidade: "Curitiba"}}

	result, err := Race(context.Background(), "01153000", broken, ok)

	assert.NoError(t, err)
	assert.Equal(t, "Boa", result.Provider)
}

func TestRaceTimeout(t *testing.T) {
	slow1 := providerMock{name: "Lenta1", delay: time.Second}
	slow2 := providerMock{name: "Lenta2", delay: time.Second}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := Race(ctx, "01153000", slow1, slow2)

	assert.ErrorIs(t, err, ErrTimeout)
	assert.Less(t, time.Since(start), 500*time.Millisecond, "Race deveria abortar junto com o contexto")
}

// com o prazo já vencido as duas falham na hora, e o resultado tem que ser
// timeout mesmo quando o select pega as falhas antes do ctx.Done()
func TestRaceFalhasPorPrazoViramTimeout(t *testing.T) {
	a := providerMock{name: "A", delay: time.Hour}
	b := providerMock{name: "B", delay: time.Hour}

	ctx, cancel := context.WithTimeout(context.Background(), 0)
	defer cancel()

	_, err := Race(ctx, "01153000", a, b)

	assert.ErrorIs(t, err, ErrTimeout)
}

func TestRaceTodasFalham(t *testing.T) {
	a := providerMock{name: "A", delay: time.Millisecond, err: ErrCEPNaoEncontrado}
	b := providerMock{name: "B", delay: 2 * time.Millisecond, err: ErrCEPNaoEncontrado}

	_, err := Race(context.Background(), "00000000", a, b)

	assert.ErrorIs(t, err, ErrCEPNaoEncontrado)
	assert.Contains(t, err.Error(), "todas as APIs falharam")
}

func TestRaceSemProvedores(t *testing.T) {
	_, err := Race(context.Background(), "01153000")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "nenhum provedor")
}
