<script lang="ts">
	import { renderMarkdown } from '$lib/markdown';

	interface Props {
		content: string;
	}

	let { content }: Props = $props();
	let html = $state('');

	$effect(() => {
		if (!content) {
			html = '';
			return;
		}

		renderMarkdown(content)
			.then((result) => {
				html = result;
			})
			.catch(() => {
				html = '';
			});
	});
</script>

{#if html}
	<div class="markdown">
		{@html html}
	</div>
{:else if content}
	<div class="text-text-secondary whitespace-pre-wrap">{content}</div>
{/if}
