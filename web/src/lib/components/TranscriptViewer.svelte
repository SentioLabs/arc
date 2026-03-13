<script lang="ts">
	interface TranscriptMessage {
		role?: string;
		type?: string;
		content?: unknown;
		tool_name?: string;
		tool_use_id?: string;
		name?: string;
		input?: unknown;
		result?: unknown;
		// eslint-disable-next-line @typescript-eslint/no-explicit-any
		[key: string]: any;
	}

	interface Props {
		transcript: TranscriptMessage[];
	}

	let { transcript }: Props = $props();

	let expandedTools = $state<Record<number, boolean>>({});

	function toggleTool(index: number) {
		expandedTools = { ...expandedTools, [index]: !expandedTools[index] };
	}

	function isToolUse(msg: TranscriptMessage): boolean {
		return msg.type === 'tool_use' || msg.type === 'tool_result' || !!msg.tool_name || !!msg.tool_use_id;
	}

	function getToolName(msg: TranscriptMessage): string {
		return msg.tool_name ?? msg.name ?? 'Unknown Tool';
	}

	function formatContent(content: unknown): string {
		if (content === null || content === undefined) return '';
		if (typeof content === 'string') return content;
		if (Array.isArray(content)) {
			return content
				.map((block) => {
					if (typeof block === 'string') return block;
					if (block?.type === 'text') return block.text ?? '';
					if (block?.type === 'tool_use') return `[Tool: ${block.name}]`;
					if (block?.type === 'tool_result') return block.content ?? '';
					return JSON.stringify(block, null, 2);
				})
				.join('\n');
		}
		return JSON.stringify(content, null, 2);
	}

	function getContentBlocks(msg: TranscriptMessage): { text: string; tools: TranscriptMessage[] } {
		const text: string[] = [];
		const tools: TranscriptMessage[] = [];

		if (Array.isArray(msg.content)) {
			for (const block of msg.content) {
				if (block?.type === 'tool_use') {
					tools.push(block);
				} else if (block?.type === 'text' && block.text) {
					text.push(block.text);
				} else if (typeof block === 'string') {
					text.push(block);
				}
			}
		} else if (msg.content) {
			text.push(formatContent(msg.content));
		}

		return { text: text.join('\n'), tools };
	}
</script>

{#if !transcript || transcript.length === 0}
	<div class="card p-8 text-center">
		<p class="text-text-muted">No transcript data available</p>
	</div>
{:else}
	<div class="space-y-4">
		{#each transcript as msg, index (index)}
			{@const role = msg.role ?? ''}
			{@const blocks = getContentBlocks(msg)}

			{#if role === 'system'}
				<!-- System messages: subtle styling -->
				<div class="px-4 py-2 text-xs text-text-muted bg-surface-800/50 rounded border border-border/50">
					<span class="font-medium uppercase tracking-wider">System</span>
					<div class="mt-1 whitespace-pre-wrap">{blocks.text}</div>
				</div>
			{:else if role === 'user'}
				<!-- User messages: left-aligned, distinct color -->
				<div class="flex justify-start">
					<div class="max-w-[85%] rounded-lg px-4 py-3 bg-surface-700 border border-border">
						<div class="text-xs text-text-muted mb-1 font-medium">User</div>
						<div class="text-sm text-text-primary whitespace-pre-wrap">{blocks.text}</div>
					</div>
				</div>
			{:else if role === 'assistant'}
				<!-- Assistant messages: right-aligned, different color -->
				<div class="flex justify-end">
					<div class="max-w-[85%] rounded-lg px-4 py-3 bg-primary-400/10 border border-primary-400/20">
						<div class="text-xs text-primary-400 mb-1 font-medium">Assistant</div>
						{#if blocks.text}
							<div class="text-sm text-text-primary whitespace-pre-wrap">{blocks.text}</div>
						{/if}

						<!-- Inline tool use blocks -->
						{#each blocks.tools as tool, toolIdx}
							{@const toolKey = index * 1000 + toolIdx}
							{@const expanded = expandedTools[toolKey] ?? false}
							<div class="mt-2 border border-border/50 rounded bg-surface-800/50">
								<button
									class="w-full flex items-center gap-2 px-3 py-2 text-xs font-mono text-text-secondary hover:text-text-primary transition-colors"
									onclick={() => toggleTool(toolKey)}
								>
									<span class="text-text-muted">{expanded ? '▼' : '▶'}</span>
									<span class="font-medium">{getToolName(tool)}</span>
									<span class="text-text-muted ml-auto">{expanded ? 'collapse' : 'expand'}</span>
								</button>
								{#if expanded}
									<div class="px-3 pb-3 border-t border-border/50">
										{#if tool.input}
											<div class="mt-2">
												<div class="text-xs text-text-muted mb-1">Input</div>
												<pre class="text-xs bg-surface-900 rounded p-2 overflow-x-auto font-mono text-text-secondary">{JSON.stringify(tool.input, null, 2)}</pre>
											</div>
										{/if}
										{#if tool.result !== undefined}
											<div class="mt-2">
												<div class="text-xs text-text-muted mb-1">Result</div>
												<pre class="text-xs bg-surface-900 rounded p-2 overflow-x-auto font-mono text-text-secondary">{formatContent(tool.result)}</pre>
											</div>
										{/if}
									</div>
								{/if}
							</div>
						{/each}
					</div>
				</div>
			{:else if isToolUse(msg)}
				<!-- Standalone tool use/result messages -->
				{@const toolKey = index}
				{@const expanded = expandedTools[toolKey] ?? false}
				<div class="border border-border/50 rounded bg-surface-800/50">
					<button
						class="w-full flex items-center gap-2 px-3 py-2 text-xs font-mono text-text-secondary hover:text-text-primary transition-colors"
						onclick={() => toggleTool(toolKey)}
					>
						<span class="text-text-muted">{expanded ? '▼' : '▶'}</span>
						<span class="font-medium">{getToolName(msg)}</span>
						<span class="text-text-muted ml-auto">{expanded ? 'collapse' : 'expand'}</span>
					</button>
					{#if expanded}
						<div class="px-3 pb-3 border-t border-border/50">
							{#if msg.input}
								<div class="mt-2">
									<div class="text-xs text-text-muted mb-1">Input</div>
									<pre class="text-xs bg-surface-900 rounded p-2 overflow-x-auto font-mono text-text-secondary">{JSON.stringify(msg.input, null, 2)}</pre>
								</div>
							{/if}
							{#if msg.content !== undefined}
								<div class="mt-2">
									<div class="text-xs text-text-muted mb-1">Result</div>
									<pre class="text-xs bg-surface-900 rounded p-2 overflow-x-auto font-mono text-text-secondary">{formatContent(msg.content)}</pre>
								</div>
							{/if}
						</div>
					{/if}
				</div>
			{:else}
				<!-- Unknown role: render generically -->
				<div class="px-4 py-3 rounded border border-border bg-surface-800/30">
					<div class="text-xs text-text-muted mb-1">{role || 'Message'}</div>
					<div class="text-sm text-text-secondary whitespace-pre-wrap">{formatContent(msg.content)}</div>
				</div>
			{/if}
		{/each}
	</div>
{/if}
