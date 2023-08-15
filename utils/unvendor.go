package utils

import "strings"

func Unvendor(symbol string) (unvendored string) {
	vendorDir := "/vendor/"
	i := strings.Index(symbol, vendorDir)
	if i == -1 {
		return symbol
	}
	return symbol[i+len(vendorDir):]
}
