package main

import (
	rfsm "github.com/ethan/rfsm/src"
)

func DefineFiatCryptoFlow() *rfsm.Definition {
	// Composite groups: FIAT / HEDGE / CRYPTO with single internal state
	fiatSub, _ := rfsm.NewDef("FIAT_SUB").
		State("fiat_internal", rfsm.WithInitial(), rfsm.WithFinal()).
		Current("fiat_internal").
		Build()

	hedgeSub, _ := rfsm.NewDef("HEDGE_SUB").
		State("hedge_internal", rfsm.WithInitial(), rfsm.WithFinal()).
		Current("hedge_internal").
		Build()

	cryptoSub, _ := rfsm.NewDef("CRYPTO_SUB").
		State("crypto_internal", rfsm.WithInitial(), rfsm.WithFinal()).
		Current("crypto_internal").
		Build()

	def, _ := rfsm.NewDef("FiatCryptoFlow").
		// Terminal states
		State("SUCCESS", rfsm.WithFinal()).
		State("FAILED", rfsm.WithFinal()).
		State("REFUNDED", rfsm.WithFinal()).
		State("EXPIRED", rfsm.WithFinal()).
		// Groups
		State("FIAT", rfsm.WithSubDef(fiatSub)).
		State("HEDGE", rfsm.WithSubDef(hedgeSub)).
		State("CRYPTO", rfsm.WithSubDef(cryptoSub)).
		// Other states
		State("INIT", rfsm.WithInitial()).
		State("PENDING_FIAT_DEPOSIT").
		State("PENDING_FIAT_EXPIRED").
		State("PENDING_FIAT_REFUND").
		State("PENDING_FIAT_DEPOSITED").
		State("PENDING_FIAT_DEPOSIT_FAILED").
		State("PENDING_HEDGE_REQUOTE").
		State("PENDING_HEDGE_EXECUTED").
		State("PENDING_HEDGE_FAILED").
		State("PENDING_HEDGE_UNWIND").
		State("PENDING_CRYPTO_WITHDRAW").
		State("PENDING_CRYPTO_WITHDRAW_FAILED").
		State("PENDING_CRYPTO_WITHDRAWN").
		Current("INIT").

		// ---- FIAT Stage ----
		On("start_fiat", rfsm.WithFrom("PENDING_FIAT_DEPOSIT"), rfsm.WithTo("FIAT")).
		On("overdue", rfsm.WithFrom("FIAT"), rfsm.WithTo("PENDING_FIAT_EXPIRED")).
		On("manual_refund", rfsm.WithFrom("FIAT"), rfsm.WithTo("PENDING_FIAT_REFUND")).
		On("success", rfsm.WithFrom("FIAT"), rfsm.WithTo("PENDING_FIAT_DEPOSITED")).
		On("failed", rfsm.WithFrom("FIAT"), rfsm.WithTo("PENDING_FIAT_DEPOSIT_FAILED")).
		On("refund", rfsm.WithFrom("PENDING_FIAT_REFUND"), rfsm.WithTo("REFUNDED")).
		On("to_failed", rfsm.WithFrom("PENDING_FIAT_DEPOSIT_FAILED"), rfsm.WithTo("FAILED")).
		On("expire", rfsm.WithFrom("PENDING_FIAT_EXPIRED"), rfsm.WithTo("EXPIRED")).

		// ---- HEDGE Stage ----
		On("to_hedge", rfsm.WithFrom("PENDING_FIAT_DEPOSITED"), rfsm.WithTo("HEDGE")).
		On("requote", rfsm.WithFrom("PENDING_HEDGE_REQUOTE"), rfsm.WithTo("HEDGE")).
		On("executed", rfsm.WithFrom("HEDGE"), rfsm.WithTo("PENDING_HEDGE_EXECUTED")).
		On("failed", rfsm.WithFrom("HEDGE"), rfsm.WithTo("PENDING_HEDGE_FAILED")).
		On("retry_hedge", rfsm.WithFrom("PENDING_HEDGE_FAILED"), rfsm.WithTo("PENDING_HEDGE_REQUOTE")).
		// Optional unwind paths
		On("revert_unwind", rfsm.WithFrom("PENDING_HEDGE_EXECUTED"), rfsm.WithTo("PENDING_HEDGE_UNWIND")).
		On("requote", rfsm.WithFrom("PENDING_HEDGE_UNWIND"), rfsm.WithTo("PENDING_HEDGE_REQUOTE")).
		On("cancel_trade", rfsm.WithFrom("PENDING_HEDGE_UNWIND"), rfsm.WithTo("PENDING_FIAT_REFUND")).

		// Proceed to crypto
		On("to_crypto", rfsm.WithFrom("PENDING_HEDGE_EXECUTED"), rfsm.WithTo("PENDING_CRYPTO_WITHDRAW")).

		// ---- CRYPTO Stage ----
		On("start_crypto", rfsm.WithFrom("PENDING_CRYPTO_WITHDRAW"), rfsm.WithTo("CRYPTO")).
		On("failed", rfsm.WithFrom("CRYPTO"), rfsm.WithTo("PENDING_CRYPTO_WITHDRAW_FAILED")).
		On("success", rfsm.WithFrom("CRYPTO"), rfsm.WithTo("PENDING_CRYPTO_WITHDRAWN")).
		On("retry", rfsm.WithFrom("PENDING_CRYPTO_WITHDRAW_FAILED"), rfsm.WithTo("CRYPTO")).
		On("unwind", rfsm.WithFrom("PENDING_CRYPTO_WITHDRAW_FAILED"), rfsm.WithTo("PENDING_HEDGE_UNWIND")).
		On("to_success", rfsm.WithFrom("PENDING_CRYPTO_WITHDRAWN"), rfsm.WithTo("SUCCESS")).

		// ---- Initial fan-in/out ----
		On("init_next", rfsm.WithFrom("INIT"), rfsm.WithTo("PENDING_FIAT_DEPOSIT")).
		Build()

	return def
}
