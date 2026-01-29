package services

import (
	"fmt"
	"math"
	"strings"
)

// NumberToWords converts a float64 amount to Spanish words with currency
// Example: 1500.50 -> "UN MIL QUINIENTOS LEMPIRAS CON 50/100"
func NumberToWords(amount float64) string {
	if amount == 0 {
		return "CERO LEMPIRAS CON 00/100"
	}

	integerPart := int64(amount)
	decimalPart := int64(math.Round((amount - float64(integerPart)) * 100))

	words := convertNumberToWords(integerPart)

	// Format: "SON: [WORDS] LEMPIRAS CON [CENTS]/100"
	return fmt.Sprintf("%s LEMPIRAS CON %02d/100", strings.ToUpper(words), decimalPart)
}

func convertNumberToWords(n int64) string {
	if n == 0 {
		return "CERO"
	}

	if n < 0 {
		return "MENOS " + convertNumberToWords(-n)
	}

	if n < 10 {
		return units[n]
	}

	if n < 30 {
		return specials[n]
	}

	if n < 100 {
		u := n % 10
		t := n / 10
		if u == 0 {
			return tens[t]
		}
		return fmt.Sprintf("%s Y %s", tens[t], units[u])
	}

	if n < 1000 {
		hundredsPart := n / 100
		remainder := n % 100
		if remainder == 0 {
			return hundreds[hundredsPart]
		}
		if hundredsPart == 1 {
			return "CIENTO " + convertNumberToWords(remainder)
		}
		return fmt.Sprintf("%s %s", hundreds[hundredsPart], convertNumberToWords(remainder))
	}

	if n < 1000000 {
		thousands := n / 1000
		remainder := n % 1000

		thousandsText := ""
		if thousands == 1 {
			thousandsText = "MIL"
		} else {
			thousandsText = convertNumberToWords(thousands) + " MIL"
		}

		if remainder == 0 {
			return thousandsText
		}
		return fmt.Sprintf("%s %s", thousandsText, convertNumberToWords(remainder))
	}

	if n < 1000000000000 {
		millions := n / 1000000
		remainder := n % 1000000

		millionsText := ""
		if millions == 1 {
			millionsText = "UN MILLÓN"
		} else {
			millionsText = convertNumberToWords(millions) + " MILLONES"
		}

		if remainder == 0 {
			return millionsText
		}
		return fmt.Sprintf("%s %s", millionsText, convertNumberToWords(remainder))
	}

	return "NÚMERO MUY GRANDE"
}

var units = []string{
	"", "UNO", "DOS", "TRES", "CUATRO", "CINCO", "SEIS", "SIETE", "OCHO", "NUEVE",
}

var specials = map[int64]string{
	10: "DIEZ", 11: "ONCE", 12: "DOCE", 13: "TRECE", 14: "CATORCE", 15: "QUINCE",
	16: "DIECISÉIS", 17: "DIECISIETE", 18: "DIECIOCHO", 19: "DIECINUEVE",
	20: "VEINTE", 21: "VEINTIUNO", 22: "VEINTIDÓS", 23: "VEINTITRÉS", 24: "VEINTICUATRO",
	25: "VEINTICINCO", 26: "VEINTISÉIS", 27: "VEINTISIETE", 28: "VEINTIOCHO", 29: "VEINTINUEVE",
}

var tens = []string{
	"", "", "VEINTE", "TREINTA", "CUARENTA", "CINCUENTA", "SESENTA", "SETENTA", "OCHENTA", "NOVENTA",
}

var hundreds = []string{
	"", "CIEN", "DOSCIENTOS", "TRESCIENTOS", "CUATROCIENTOS", "QUINIENTOS", "SEISCIENTOS", "SETECIENTOS", "OCHOCIENTOS", "NOVECIENTOS",
}
