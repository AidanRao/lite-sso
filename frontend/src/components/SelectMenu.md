# SelectMenu

统一的按钮与浮层选择器。选项格式为 `{ value, label, disabled? }`，`value` 使用字符串或数字。样式沿用现有主题变量，浮层挂载到 `body`，自动避让窗口边缘。

单选点击后立即更新：

```vue
<SelectMenu v-model="primaryEmailID" label="主邮箱" :options="emailOptions" @change="savePrimaryEmail" />
```

多选先编辑草稿，点击确定才更新绑定值并触发 `change`；取消、Esc、点击外部均丢弃草稿：

```vue
<SelectMenu
  v-model="channels"
  label="通知渠道"
  title="选择通知渠道"
  :options="[{ value: 'site', label: '站内通知' }, { value: 'email', label: '邮箱' }]"
  multiple
  confirm-text="保存"
  @change="saveChannels"
/>
```

`multiple` 模式的绑定值为数组，默认带确认操作。单选也可通过 `confirm` 开启确认操作。重复确定相同值不触发更新。`disabled` 禁用整个组件，选项的 `disabled` 禁止选择该项。`placeholder` 设置空值文案，`id` 设置触发按钮 ID，便于与外部 label 关联。

支持 Enter/Space 打开或选择、方向键与 Home/End 导航、Esc 关闭并返回按钮焦点、Tab 离开关闭。`label` 用于无障碍名称，应传入具体名称（默认“选择选项”）。
