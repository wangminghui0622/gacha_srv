package validator

import (
	"reflect"

	"github.com/go-playground/validator/v10"
	"github.com/shopspring/decimal"
)

// decimalGreaterThen 验证十进制字符串字段大于参数值
func decimalGreaterThen(fl validator.FieldLevel) bool {
	field := fl.Field()
	param := fl.Param()

	if field.Kind() == reflect.String {
		d, err := decimal.NewFromString(field.String())
		if err != nil {
			return false
		}
		p, err := decimal.NewFromString(param)
		if err != nil {
			return false
		}

		return d.GreaterThan(p)
	}
	return false
}

// decimalGreaterThenOrEqual 验证十进制字符串字段大于或等于参数值
func decimalGreaterThenOrEqual(fl validator.FieldLevel) bool {
	field := fl.Field()
	param := fl.Param()

	if field.Kind() == reflect.String {
		d, err := decimal.NewFromString(field.String())
		if err != nil {
			return false
		}
		p, err := decimal.NewFromString(param)
		if err != nil {
			return false
		}

		return d.GreaterThanOrEqual(p)
	}
	return false
}

// decimalGreaterThenField 验证十进制字符串字段大于同结构体中另一字段
func decimalGreaterThenField(fl validator.FieldLevel) bool {
	field := fl.Field()
	kind := field.Kind()

	currentField, currentKind, _, ok := fl.GetStructFieldOK2()
	if !ok || currentKind != kind {
		return false
	}

	if field.Type() != currentField.Type() {
		return false
	}

	if kind == reflect.String {
		d, err := decimal.NewFromString(field.String())
		if err != nil {
			return false
		}
		p, err := decimal.NewFromString(currentField.String())
		if err != nil {
			return false
		}

		return d.GreaterThan(p)
	}
	return false
}

// decimalGreaterThenOrEqualField 验证十进制字符串字段大于或等于同结构体中另一字段
func decimalGreaterThenOrEqualField(fl validator.FieldLevel) bool {
	field := fl.Field()
	kind := field.Kind()

	currentField, currentKind, _, ok := fl.GetStructFieldOK2()
	if !ok || currentKind != kind {
		return false
	}

	if field.Type() != currentField.Type() {
		return false
	}

	if kind == reflect.String {
		d, err := decimal.NewFromString(field.String())
		if err != nil {
			return false
		}
		p, err := decimal.NewFromString(currentField.String())
		if err != nil {
			return false
		}

		return d.GreaterThanOrEqual(p)
	}
	return false
}
