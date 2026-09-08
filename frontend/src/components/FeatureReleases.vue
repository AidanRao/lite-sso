<template>
  <section class="releases" aria-label="功能发布">
    <header><div><h2>功能发布</h2><p>调整开放范围和 Beta 标签，保存后生效。</p></div>
      <button type="button" :disabled="loading || saving" @click="load">刷新</button></header>
    <p v-if="loading" role="status">正在加载功能配置…</p>
    <p v-else-if="error" role="alert">{{ error }} <button type="button" @click="load">重试</button></p>
    <template v-else>
      <form v-for="item in features" :key="item.key" @submit.prevent="save(item)">
        <fieldset :disabled="saving">
          <legend>{{ item.name }}</legend>
          <label>开放范围
            <select v-model="item.audience">
              <option value="off">关闭</option><option value="selected">指定用户</option>
              <option value="percentage">百分比灰度</option><option value="all">全量开放</option>
            </select>
          </label>
          <label v-if="item.audience === 'percentage'">灰度百分比
            <input v-model.number="item.percentage" type="number" min="0" max="100" step="1" required />
            <small>同一用户的分配保持稳定，扩大比例保留已命中用户；实际人数可能与比例略有差异。</small>
          </label>
          <div v-if="['selected', 'percentage'].includes(item.audience)" class="audience">
            <label>指定用户（已选 {{ item.user_ids.length }} 人）
              <input v-model="search" type="search" placeholder="搜索用户名、邮箱或用户 ID" />
            </label>
            <p v-if="item.audience === 'percentage'">指定用户始终命中当前灰度，无需满足百分比。</p>
            <div class="user-list">
              <label v-for="user in filteredUsers" :key="user.id" class="user-option">
                <input v-model="item.user_ids" type="checkbox" :value="user.id" />
                <span>{{ user.username || user.email || '未命名用户' }}<small>{{ user.email }} · {{ user.id }}</small></span>
              </label>
              <p v-if="!filteredUsers.length">没有匹配的用户</p>
            </div>
          </div>
          <label class="beta-option"><input v-model="item.stage" type="checkbox" true-value="beta" false-value="stable" />显示 Beta 标签</label>
          <button type="submit">{{ saving ? '正在保存…' : '保存配置' }}</button>
        </fieldset>
      </form>
      <p v-if="notice" role="status">{{ notice }}</p>
    </template>
  </section>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { adminAPI } from '../api/auth'
const features = ref([])
const users = ref([])
const search = ref('')
const loading = ref(true)
const saving = ref(false)
const error = ref('')
const notice = ref('')
const filteredUsers = computed(() => {
  const query = search.value.trim().toLocaleLowerCase()
  return users.value.filter(user => [user.username, user.email, user.id].some(value => String(value || '').toLocaleLowerCase().includes(query)))
})
async function load() {
  loading.value = true
  error.value = ''
  notice.value = ''
  try {
    const [policies, audience] = await Promise.all([adminAPI.getFeatures(), adminAPI.listUsers()])
    features.value = policies.data.features
    users.value = audience.data.users
  } catch (err) { error.value = err.message || '加载失败' }
  finally { loading.value = false }
}
async function save(item) {
  saving.value = true
  notice.value = ''
  try {
    await adminAPI.updateFeature(item.key, { audience: item.audience, percentage: item.percentage, stage: item.stage, user_ids: item.user_ids })
    notice.value = '功能发布配置已保存'
  } catch (err) { notice.value = err.message || '保存失败，请重试' }
  finally { saving.value = false }
}
onMounted(load)
</script>

<style scoped>
.releases { padding: 24px; color: #1f2328; }
header { display: flex; align-items: center; justify-content: space-between; gap: 16px; }
h2 { margin: 0; font-size: 20px; font-weight: 600; }
p, small { color: #656d76; font-size: 13px; }
form { margin-top: 24px; }
fieldset { border: 1px solid #d0d7de; border-radius: 6px; padding: 20px; display: grid; gap: 20px; min-width: 0; }
legend { font-weight: 600; }
label { display: grid; gap: 8px; font-size: 14px; }
input:not([type=checkbox]), select { padding: 8px; border: 1px solid #d0d7de; border-radius: 6px; background: white; color: inherit; width: 100%; box-sizing: border-box; }
button { padding: 7px 14px; border: 1px solid #d0d7de; border-radius: 6px; background: #f6f8fa; color: inherit; cursor: pointer; justify-self: start; }
button:disabled { opacity: .6; cursor: default; }
.user-list { max-height: 280px; overflow-y: auto; border: 1px solid #d0d7de; border-radius: 6px; padding: 8px; margin-top: 10px; }
.user-option, .beta-option { display: flex; align-items: center; gap: 10px; }
.user-option { padding: 8px; }
.user-option span { overflow-wrap: anywhere; min-width: 0; }
.user-option small { display: block; }
@media (max-width: 760px) { .releases { padding: 16px; } fieldset { padding: 12px; } }
</style>
