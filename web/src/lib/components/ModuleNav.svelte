<script lang="ts">
	import { visibleModules } from '../modules';

	let { features }: { features: readonly string[] } = $props();

	const modules = $derived(visibleModules(features));
</script>

{#if modules.length > 0}
	<nav aria-label="Modul">
		<ul>
			{#each modules as m (m.id)}
				<li>
					<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -- href berasal dari registry modul -->
					<a href={m.href}>{m.label}</a>
				</li>
			{/each}
		</ul>
	</nav>
{/if}
