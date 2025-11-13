package main

import (
	rfsm "github.com/noru/rfsm"
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
		On("start_fiat", "PENDING_FIAT_DEPOSIT", "FIAT").
		On("overdue", "FIAT", "PENDING_FIAT_EXPIRED").
		On("manual_refund", "FIAT", "PENDING_FIAT_REFUND").
		On("success", "FIAT", "PENDING_FIAT_DEPOSITED").
		On("failed", "FIAT", "PENDING_FIAT_DEPOSIT_FAILED").
		On("refund", "PENDING_FIAT_REFUND", "REFUNDED").
		On("to_failed", "PENDING_FIAT_DEPOSIT_FAILED", "FAILED").
		On("expire", "PENDING_FIAT_EXPIRED", "EXPIRED").

		// ---- HEDGE Stage ----
		On("to_hedge", "PENDING_FIAT_DEPOSITED", "HEDGE").
		On("requote", "PENDING_HEDGE_REQUOTE", "HEDGE").
		On("executed", "HEDGE", "PENDING_HEDGE_EXECUTED").
		On("failed", "HEDGE", "PENDING_HEDGE_FAILED").
		On("retry_hedge", "PENDING_HEDGE_FAILED", "PENDING_HEDGE_REQUOTE").
		// Optional unwind paths
		On("revert_unwind", "PENDING_HEDGE_EXECUTED", "PENDING_HEDGE_UNWIND").
		On("requote", "PENDING_HEDGE_UNWIND", "PENDING_HEDGE_REQUOTE").
		On("cancel_trade", "PENDING_HEDGE_UNWIND", "PENDING_FIAT_REFUND").

		// Proceed to crypto
		On("to_crypto", "PENDING_HEDGE_EXECUTED", "PENDING_CRYPTO_WITHDRAW").

		// ---- CRYPTO Stage ----
		On("start_crypto", "PENDING_CRYPTO_WITHDRAW", "CRYPTO").
		On("failed", "CRYPTO", "PENDING_CRYPTO_WITHDRAW_FAILED").
		On("success", "CRYPTO", "PENDING_CRYPTO_WITHDRAWN").
		On("retry", "PENDING_CRYPTO_WITHDRAW_FAILED", "CRYPTO").
		On("unwind", "PENDING_CRYPTO_WITHDRAW_FAILED", "PENDING_HEDGE_UNWIND").
		On("to_success", "PENDING_CRYPTO_WITHDRAWN", "SUCCESS").

		// ---- Initial fan-in/out ----
		On("init_next", "INIT", "PENDING_FIAT_DEPOSIT").
		Build()

	return def
}
