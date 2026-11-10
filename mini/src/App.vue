<script setup lang="ts">
import { onLaunch } from '@dcloudio/uni-app'

// 启动钩子：语言恒 zh-CN 与 web 一致（无持久化），登录态由登录页入口校验（token 有效 → 首页）
onLaunch(() => {
  console.log('App Launch')
})
</script>

<style lang="scss">
// 全局设计令牌与通用样式（web styles/tokens.scss 镜像，规范 06-i18n-styling）
page {
  --color-primary: #6c4cf1;
  --color-primary-dark: #5a3de0;
  --color-accent: #ff6b35;
  --color-danger: #ff4d4f;
  --color-warning: #d48806;
  --color-bg: #f5f5f5;
  --color-card: #ffffff;
  --color-border: #e8e8e8;
  --color-text: #262626;
  --color-text-secondary: #8c8c8c;
  background: var(--color-bg);
  color: var(--color-text);
  font-size: 28rpx;
}

.gacha-card {
  background: var(--color-card);
  border-radius: 16rpx;
  padding: 28rpx;
  margin: 0 24rpx 24rpx;
  box-shadow: 0 2rpx 12rpx rgba(0, 0, 0, 0.04);
}

.page-title {
  font-size: 40rpx;
  font-weight: 700;
  margin: 24rpx 24rpx 8rpx;
}

.page-subtitle {
  font-size: 26rpx;
  color: var(--color-text-secondary);
  margin: 0 24rpx 24rpx;
}

.money {
  color: var(--color-accent);
  font-weight: 600;
}

.error-text {
  color: var(--color-danger);
  text-align: center;
  margin: 24rpx;
}

.btn-primary {
  background: var(--color-primary);
  color: #fff;
  border-radius: 12rpx;
  font-size: 32rpx;
  line-height: 88rpx;
  text-align: center;

  &::after {
    border: none;
  }

  &.disabled {
    opacity: 0.5;
  }
}

.btn-plain {
  background: transparent;
  color: var(--color-primary);
  border: 1rpx solid var(--color-primary);
  border-radius: 12rpx;
  font-size: 32rpx;
  line-height: 88rpx;
  text-align: center;
  margin-top: 16rpx;

  &::after {
    border: none;
  }

  &.disabled {
    opacity: 0.5;
  }
}

// M16b 玩法页共享工具类（section 标题 / 状态标签 / 进度条 / 弹层 / 空态）
.section-title {
  font-size: 26rpx;
  font-weight: 600;
  color: var(--color-text-secondary);
  margin: 20rpx 0 12rpx;
}

.tag-chip {
  display: inline-block;
  font-size: 20rpx;
  line-height: 1.5;
  border-radius: 6rpx;
  padding: 0 10rpx;
  margin-left: 12rpx;
  background: var(--color-bg);
  color: var(--color-text-secondary);

  &.tag-green {
    color: #389e0d;
    background: rgba(56, 158, 13, 0.1);
  }

  &.tag-orange {
    color: #d46b08;
    background: rgba(212, 107, 8, 0.1);
  }

  &.tag-red {
    color: #cf1322;
    background: rgba(207, 19, 34, 0.1);
  }

  &.tag-blue {
    color: #0958d9;
    background: rgba(9, 88, 217, 0.1);
  }

  &.tag-gold {
    color: #ad6800;
    background: rgba(173, 104, 0, 0.12);
  }
}

.progress-track {
  height: 12rpx;
  background: var(--color-border);
  border-radius: 6rpx;
  overflow: hidden;

  .progress-fill {
    height: 100%;
    background: var(--color-primary);
    border-radius: 6rpx;
  }

  &.done .progress-fill {
    background: #52c41a;
  }
}

.modal-mask {
  position: fixed;
  top: 0;
  right: 0;
  bottom: 0;
  left: 0;
  background: rgba(0, 0, 0, 0.45);
  z-index: 100;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 48rpx;

  .modal-card {
    width: 100%;
    background: var(--color-card);
    border-radius: 20rpx;
    padding: 32rpx;
    max-height: 80vh;
    overflow-y: auto;
  }
}

.list-empty {
  text-align: center;
  color: var(--color-text-secondary);
  padding: 48rpx 0;
  font-size: 26rpx;
}
</style>
