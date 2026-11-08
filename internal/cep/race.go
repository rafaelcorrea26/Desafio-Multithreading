package cep

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var ErrTimeout = errors.New("timeout: nenhuma API respondeu dentro do tempo limite")

// resposta que ganhou a corrida
type Result struct {
	Provider string
	Address  Address
	Elapsed  time.Duration
}

// o que cada goroutine joga no channel
type outcome struct {
	provider string
	address  Address
	elapsed  time.Duration
	err      error
}

// Race consulta todos os providers ao mesmo tempo e retorna o primeiro que
// responder sem erro. Se o contexto estourar antes disso, devolve ErrTimeout.
func Race(ctx context.Context, cep string, providers ...Provider) (Result, error) {
	if len(providers) == 0 {
		return Result{}, errors.New("nenhum provedor informado")
	}

	// contexto filho só pra poder cancelar as requisições que perderam
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// o buffer é importante: sem ele a goroutine perdedora ficaria travada pra
	// sempre tentando escrever aqui depois que a corrida já acabou
	outcomes := make(chan outcome, len(providers))

	for _, p := range providers {
		go func() {
			start := time.Now()
			addr, err := p.Fetch(ctx, cep)
			outcomes <- outcome{
				provider: p.Name(),
				address:  addr,
				elapsed:  time.Since(start),
				err:      err,
			}
		}()
	}

	var falhas []error
	for range providers {
		select {
		case <-ctx.Done():
			if len(falhas) > 0 {
				return Result{}, fmt.Errorf("%w (erros até aqui: %w)", ErrTimeout, errors.Join(falhas...))
			}
			return Result{}, ErrTimeout

		case o := <-outcomes:
			// erro não ganha a corrida: guarda e continua esperando a outra API
			if o.err != nil {
				falhas = append(falhas, fmt.Errorf("%s: %w", o.provider, o.err))
				continue
			}
			return Result{Provider: o.provider, Address: o.address, Elapsed: o.elapsed}, nil
		}
	}

	// as falhas podem ter sido causadas pelo prazo e chegado antes do ctx.Done()
	if ctx.Err() != nil {
		return Result{}, fmt.Errorf("%w (erros até aqui: %w)", ErrTimeout, errors.Join(falhas...))
	}
	return Result{}, fmt.Errorf("todas as APIs falharam: %w", errors.Join(falhas...))
}
