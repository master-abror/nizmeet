<script lang="ts">
  import type { Snippet } from 'svelte';

  interface Props {
    children?: Snippet;
    onclick?: (event: MouseEvent) => void;
    variant?: 'primary' | 'ghost';
    type?: 'button' | 'submit';
  }

  let { children, onclick, variant = 'primary', type = 'button' }: Props = $props();
</script>

<button {type} {onclick} class="ui-btn ui-btn--{variant}">
  {@render children?.()}
</button>

<style>
  .ui-btn {
    font-family: var(--ui-font, system-ui, sans-serif);
    border-radius: var(--ui-radius, 0.5rem);
    padding: 0.5rem 1rem;
    border: 1px solid transparent;
    cursor: pointer;
  }
  .ui-btn--primary {
    background: var(--ui-brand, #4f46e5);
    color: var(--ui-brand-contrast, #fff);
  }
  .ui-btn--ghost {
    background: transparent;
    color: var(--ui-brand, #4f46e5);
    border-color: var(--ui-brand, #4f46e5);
  }
</style>
