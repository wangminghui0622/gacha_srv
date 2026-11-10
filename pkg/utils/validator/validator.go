// Package validator 网关全局请求校验器（规范 03-api-proto 规则 7、05-errors 规则 10）：
// gateway 启动时 httpx.SetValidator(validator.New()) 注册，go-zero 在 httpx.Parse
// 中对 JSON/form/path 三种解析模式统一回调 Validate；
// 校验失败返回 CodeParamError(400) 与中文提示（取第一条），不允许被吞成 500。
//
// 实现参考 slots_platform/pkg/utils/validator（go-playground/validator/v10 +
// universal-translator 中文翻译 + 字段友好名映射 + 自定义校验器）。
// 注意：min/max 翻译按「长度」语义（适合字符串/切片），数值字段请用 gte/lte/gt/lt。
// date.go 的时间类校验器（datetime/dltField/tbfField）依赖 timex 包，Gacha 暂无该依赖，暂未移植。
package validator

import (
	"fmt"
	"net/http"
	"reflect"
	"regexp"
	"strings"

	"gacha_srv/pkg/utils/errorx"

	"github.com/go-playground/locales/zh"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	zh_translations "github.com/go-playground/validator/v10/translations/zh"
)

var (
	// regStr 用户名/密码/昵称等允许的字符集：中文、字母、数字及 @_!&*
	regStr = "^[一-龥a-zA-Z0-9@_!&*]+$"

	// 字段友好名称映射：错误提示里展示中文名，不在映射内的字段
	// 回退到 json tag（与 .api 定义一致），再无则用 Go 字段名
	fieldFriendlyNames = map[string]string{
		// 认证相关
		"Username": "用户名",
		"Password": "密码",
		"Nickname": "昵称",

		// 支付相关
		"Amount": "金额",

		// 游戏相关
		"BoxId": "宝箱ID",

		// 抽赏记录
		"RequestId": "请求ID",

		// 分页
		"Page":     "页码",
		"PageSize": "每页数量",
	}
)

type ParamValidator struct {
	validator *validator.Validate
	trans     ut.Translator
	active    bool
}

var defaultValidators = map[string]validator.Func{
	"name":           regexpValidator(regStr),
	"passwordFormat": passwordFormat,
	"deField":        stringEqual,
	"dgt":            decimalGreaterThen,
	"dgte":           decimalGreaterThenOrEqual,
	"dgtField":       decimalGreaterThenField,
	"dgteField":      decimalGreaterThenOrEqualField,
}

func New() *ParamValidator {
	pv := &ParamValidator{
		validator: validator.New(),
		active:    true,
	}

	// 初始化翻译器（中文）
	zhLocale := zh.New()
	uni := ut.New(zhLocale, zhLocale)
	trans, _ := uni.GetTranslator("zh")
	pv.trans = trans

	// 注册默认翻译
	if err := zh_translations.RegisterDefaultTranslations(pv.validator, trans); err != nil {
		panic(fmt.Sprintf("注册默认翻译失败: %v", err))
	}

	// 注册自定义翻译（覆盖默认文案，统一为「xxx不能…」句式）
	if err := registerCustomTranslations(pv.validator, trans); err != nil {
		panic(fmt.Sprintf("注册自定义翻译失败: %v", err))
	}

	// 注册自定义验证器
	for tag, val := range defaultValidators {
		if err := pv.validator.RegisterValidation(tag, val); err != nil {
			panic(fmt.Sprintf("注册验证器 %s 失败: %v", tag, err))
		}
	}

	// 注册字段名称翻译（友好名映射；未映射的字段回退 Go 字段名）
	pv.validator.RegisterTagNameFunc(func(fld reflect.StructField) string {
		if friendlyName, exists := fieldFriendlyNames[fld.Name]; exists {
			return friendlyName
		}
		return fld.Name
	})

	return pv
}

// registerCustomTranslations 注册自定义错误消息翻译
func registerCustomTranslations(v *validator.Validate, trans ut.Translator) error {
	translations := map[string]string{
		"min":            "{0}长度不能少于{1}",
		"max":            "{0}长度不能超过{1}",
		"required":       "{0}不能为空",
		"email":          "{0}格式不正确",
		"numeric":        "{0}必须是数字",
		"oneof":          "{0}必须是以下值之一: {1}",
		"gte":            "{0}必须大于或等于{1}",
		"lte":            "{0}必须小于或等于{1}",
		"gt":             "{0}必须大于{1}",
		"lt":             "{0}必须小于{1}",
		"len":            "{0}长度必须为{1}",
		"name":           "{0}格式不正确，只能包含中文、字母、数字和特殊字符",
		"deField":        "两次密码必须相同",
		"dgt":            "{0}必须大于{1}",
		"dgte":           "{0}必须大于或等于{1}",
		"passwordFormat": "{0}格式不正确，只能包含中文、字母、数字和特殊字符",
	}

	for tag, message := range translations {
		if err := v.RegisterTranslation(tag, trans,
			func(ut ut.Translator) error {
				return ut.Add(tag, message, true)
			},
			func(ut ut.Translator, fe validator.FieldError) string {
				t, _ := ut.T(tag, getFieldFriendlyName(fe.StructField()), fe.Param())
				return t
			},
		); err != nil {
			return err
		}
	}

	return nil
}

// getFieldFriendlyName 获取字段的友好名称：从完整路径（可能带嵌套前缀）中
// 提取叶子字段名，查友好名映射，未映射则原样返回
func getFieldFriendlyName(field string) string {
	parts := strings.Split(field, ".")
	fieldName := parts[len(parts)-1]

	if friendlyName, exists := fieldFriendlyNames[fieldName]; exists {
		return friendlyName
	}
	return fieldName
}

// Validate 验证结构体并返回友好的错误信息（实现 httpx.Validator 接口）
func (pv *ParamValidator) Validate(r *http.Request, val any) error {
	if !pv.active {
		return nil
	}

	err := pv.validator.Struct(val)
	if err != nil {
		if validationErrors, ok := err.(validator.ValidationErrors); ok {
			if len(validationErrors) > 0 {
				// 返回第一个错误的友好提示（业务码为 400 参数错误，
				// 避免被 errorx.FromError 兜底成 500 服务器错误）
				firstError := validationErrors[0]
				return errorx.NewCodeError(errorx.CodeParamError, firstError.Translate(pv.trans))
			}
		}
		return errorx.NewCodeError(errorx.CodeParamError, fmt.Sprintf("参数验证失败: %v", err))
	}

	return nil
}

// ValidateAll 验证并返回所有错误信息（field 友好名 → 中文提示）
func (pv *ParamValidator) ValidateAll(val any) map[string]string {
	errors := make(map[string]string)

	if !pv.active {
		return errors
	}

	err := pv.validator.Struct(val)
	if err != nil {
		if validationErrors, ok := err.(validator.ValidationErrors); ok {
			for _, ve := range validationErrors {
				errors[getFieldFriendlyName(ve.StructField())] = ve.Translate(pv.trans)
			}
		}
	}

	return errors
}

// SetActive 设置验证器是否激活
func (pv *ParamValidator) SetActive(active bool) {
	pv.active = active
}

// AddCustomValidator 添加自定义验证器
func (pv *ParamValidator) AddCustomValidator(tag string, fn validator.Func) error {
	return pv.validator.RegisterValidation(tag, fn)
}

// AddFieldFriendlyName 添加字段友好名称
func (pv *ParamValidator) AddFieldFriendlyName(field, friendlyName string) {
	fieldFriendlyNames[field] = friendlyName
}

// regexpValidator 正则表达式验证器
func regexpValidator(pattern string) validator.Func {
	reg := regexp.MustCompile(pattern)
	return func(fl validator.FieldLevel) bool {
		return reg.MatchString(fl.Field().String())
	}
}
