<script lang="ts" setup>
import { computed } from 'vue'
import { renderMarkdown } from '../../utils/markdown'

const props = defineProps<{
  content: string
}>()

// Reuses the app's existing marked + highlight.js renderer (code blocks, copy
// buttons). Styling comes from the global `.md-preview` + `.code-block-wrapper` rules.
const html = computed(() => renderMarkdown(props.content))
</script>

<template>
  <div class="md-preview p-6" v-html="html" />
</template>
