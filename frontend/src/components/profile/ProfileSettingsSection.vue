<template>
  <section class="profile-settings-section" :aria-labelledby="titleId">
    <header class="profile-section-title" :class="{ 'has-actions': $slots.actions }">
      <h2 :id="titleId">{{ title }}</h2>
      <div v-if="$slots.actions" class="profile-section-actions">
        <slot name="actions" />
      </div>
    </header>
    <p v-if="description" class="profile-section-copy">{{ description }}</p>
    <slot />
  </section>
</template>

<script setup>
defineProps({
  titleId: {
    type: String,
    required: true
  },
  title: {
    type: String,
    required: true
  },
  description: {
    type: String,
    default: ''
  }
})
</script>

<style scoped>
.profile-settings-section {
  display: grid;
  gap: 14px;
}

.profile-section-title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 18px;
  padding-bottom: 8px;
  border-bottom: 1px solid var(--profile-divider);
}

.profile-section-title h2 {
  margin: 0;
  color: var(--profile-text-strong);
  font-size: 20px;
  font-weight: 600;
  line-height: 1.5;
}

.profile-section-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.profile-section-copy {
  margin: 0;
  color: var(--profile-text-muted);
  font-size: 14px;
  line-height: 1.5;
}

@media (max-width: 620px) {
  .profile-section-title {
    align-items: flex-start;
  }

  .profile-section-title.has-actions {
    flex-direction: column;
  }
}
</style>
