<template>
  <button @click="submit">{{ label }}</button>
</template>

<script setup lang="ts">
import { parseResponse } from "./helpers"

const label = "Ready"

function submit(): string {
  return parseResponse(label)
}
</script>
