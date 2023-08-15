package utils

import "github.com/pkg/errors"

func True(c bool) {
	if !c {
		panic(errors.New("assert: unexpected false value"))
	}
}

func False(c bool) {
	if c {
		panic(errors.New("assert: unexpected true value"))
	}
}

func NoError(err error) {
	if err != nil {
		panic(errors.Wrap(err, "unexpected error"))
	}
}

func NotNil(v ...interface{}) {
	for _, v := range v {
		if v == nil {
			panic(errors.New("assert: unexpected nil value"))
		}
	}
}
