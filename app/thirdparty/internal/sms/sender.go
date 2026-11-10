// Sender 照抄 server_v2 integration/sms/sender.go：阿里云/腾讯双通道、
// DryRun 跳过、888 前缀测试号跳过阿里云、两通道都关且非 DryRun → ErrSmsDisabled。
package sms

import (
	"context"
	"fmt"
	"strings"

	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	alisms "github.com/alibabacloud-go/dysmsapi-20170525/v3/client"
	"github.com/alibabacloud-go/tea/tea"
	tcc "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/regions"
	tcsms "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/sms/v20210111"
)

type AliyunSMS struct {
	Enabled         bool
	AccessKeyID     string
	AccessKeySecret string
	SignName        string
	TemplateCode    string
}

type TencentSMS struct {
	Enabled         bool
	AccessKeyID     string
	AccessKeySecret string
	SignName        string
	TemplateID      string
	AppID           string
}

type SMSConfig struct {
	Aliyun  AliyunSMS
	Tencent TencentSMS
	DryRun  bool
}

type Sender struct {
	cfg     SMSConfig
	ali     *alisms.Client
	tencent *tcsms.Client
}

func NewSender(cfg SMSConfig) (*Sender, error) {
	s := &Sender{cfg: cfg}
	if cfg.Aliyun.Enabled {
		config := &openapi.Config{AccessKeyId: tea.String(cfg.Aliyun.AccessKeyID), AccessKeySecret: tea.String(cfg.Aliyun.AccessKeySecret)}
		config.Endpoint = tea.String("dysmsapi.aliyuncs.com")
		client, err := alisms.NewClient(config)
		if err != nil {
			return nil, err
		}
		s.ali = client
	}
	if cfg.Tencent.Enabled {
		credential := tcc.NewCredential(cfg.Tencent.AccessKeyID, cfg.Tencent.AccessKeySecret)
		cpf := profile.NewClientProfile()
		cpf.SignMethod = tcc.SHA256
		client, err := tcsms.NewClient(credential, regions.Guangzhou, cpf)
		if err != nil {
			return nil, err
		}
		s.tencent = client
	}
	return s, nil
}

func (s *Sender) Send(ctx context.Context, phone, code string) error {
	if s.cfg.DryRun {
		return nil
	}
	if !s.cfg.Aliyun.Enabled && !s.cfg.Tencent.Enabled {
		return ErrSmsDisabled
	}
	var errs []error
	if s.cfg.Aliyun.Enabled && s.ali != nil && !strings.HasPrefix(phone, "888") {
		req := &alisms.SendSmsRequest{
			PhoneNumbers: tea.String(phone), SignName: tea.String(s.cfg.Aliyun.SignName),
			TemplateCode:  tea.String(s.cfg.Aliyun.TemplateCode),
			TemplateParam: tea.String(fmt.Sprintf(`{"code":"%s"}`, code)),
		}
		if _, err := s.ali.SendSms(req); err != nil {
			errs = append(errs, err)
		}
	}
	if s.cfg.Tencent.Enabled && s.tencent != nil {
		req := tcsms.NewSendSmsRequest()
		req.SmsSdkAppId = tcc.StringPtr(s.cfg.Tencent.AppID)
		req.SignName = tcc.StringPtr(s.cfg.Tencent.SignName)
		req.TemplateId = tcc.StringPtr(s.cfg.Tencent.TemplateID)
		req.PhoneNumberSet = tcc.StringPtrs([]string{"+86" + phone})
		req.TemplateParamSet = tcc.StringPtrs([]string{code})
		if _, err := s.tencent.SendSmsWithContext(ctx, req); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}
