<script lang="ts">
	import './layout.css';
	import favicon from '#lib/assets/favicon.svg';
	import ModuleNav from '#lib/components/ModuleNav.svelte';
	import type { LayoutProps } from './$types';

	let { children, data }: LayoutProps = $props();
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
</svelte:head>

<ModuleNav features={data.features} />

{@render children()}
