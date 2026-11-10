package validator

import (
	"reflect"
	"regexp"

	"github.com/go-playground/validator/v10"
)

// passwordFormat 只验证密码格式（与用户名共用字符集）
func passwordFormat(fl validator.FieldLevel) bool {
	if fl.Field().Kind() != reflect.String {
		return false
	}
	pwdRegex := regexp.MustCompile(regStr)
	return pwdRegex.MatchString(fl.Field().String())
}

// stringEqual 验证字段与同一结构体中指定字段相等（如确认密码）
func stringEqual(fl validator.FieldLevel) bool {
	field := fl.Field()
	if field.Kind() != reflect.String {
		return false
	}

	fieldName := fl.Param()
	if fieldName == "" {
		return false
	}

	currentField, currentKind, _, ok := fl.GetStructFieldOKAdvanced2(fl.Parent(), fieldName)
	if !ok || currentKind != reflect.String {
		return false
	}

	return field.String() == currentField.String()
}
