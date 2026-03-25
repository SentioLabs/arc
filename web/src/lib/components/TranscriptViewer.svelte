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
		[key: string]: unknown;
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

	function roleLabel(role: string): string {
		switch (role) {
			case 'user':
				return 'User';
			case 'assistant':
				return 'Assistant';
			case 'system':
				return 'System';
			default:
				return role || 'Message';
		}
	}

	function roleBorderColor(role: string): string {
		switch (role) {
			case 'user':
				return 'border-l-blue-400';
			case 'assistant':
				return 'border-l-primary-400';
			case 'system':
				return 'border-l-text-muted';
			default:
				return 'border-l-border';
		}
	}

	function roleLabelColor(role: string): string {
		switch (role) {
			case 'user':
				return 'text-blue-400';
			case 'assistant':
				return 'text-primary-400';
			case 'system':
				return 'text-text-muted';
			default:
				return 'text-text-muted';
		}
	}
</script>

{#if !transcript || transcript.length === 0}
	<div class="card p-8 text-center">
		<p class="text-text-muted">No transcript data available</p>
	</div>
{:else}
	<div class="space-y-1">
		{#each transcript as msg, index (index)}
			{@const role = msg.role ?? ''}
			{@const blocks = getContentBlocks(msg)}

			{#if isToolUse(msg) && !role}
				<!-- Standalone tool use/result (no role) -->
				{@const toolKey = index}
				{@const expanded = expandedTools[toolKey] ?? false}
				<div class="border-l-2 border-l-amber-500/60 pl-4 py-2">
					<button
						class="flex items-center gap-2 text-xs font-mono text-text-secondary hover:text-text-primary transition-colors"
						onclick={() => toggleTool(toolKey)}
					>
						<span class="text-text-muted">{expanded ? '▼' : '▶'}</span>
						<span class="text-amber-500/80 font-medium">{getToolName(msg)}</span>
					</button>
					{#if expanded}
						<div class="mt-2 ml-4">
							{#if msg.input}
								<div class="mb-2">
									<div class="text-xs text-text-muted mb-1">Input</div>
									<pre class="text-xs bg-surface-900 rounded p-2 overflow-x-auto font-mono text-text-secondary max-h-96">{JSON.stringify(msg.input, null, 2)}</pre>
								</div>
							{/if}
							{#if msg.content !== undefined}
								<div>
									<div class="text-xs text-text-muted mb-1">Result</div>
									<pre class="text-xs bg-surface-900 rounded p-2 overflow-x-auto font-mono text-text-secondary max-h-96">{formatContent(msg.content)}</pre>
								</div>
							{/if}
						</div>
					{/if}
				</div>
			{:else}
				<!-- Standard message (user, assistant, system) -->
				<div class="border-l-2 {roleBorderColor(role)} pl-4 py-2">
					<div class="text-xs {roleLabelColor(role)} font-medium mb-1">{roleLabel(role)}</div>

					{#if role === 'system'}
						<div class="text-xs text-text-muted whitespace-pre-wrap">{blocks.text}</div>
					{:else}
						{#if blocks.text}
							<div class="text-sm text-text-primary whitespace-pre-wrap">{blocks.text}</div>
						{/if}
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
								<span class="text-amber-500/80 font-medium">{getToolName(tool)}</span>
							</button>
							{#if expanded}
								<div class="px-3 pb-3 border-t border-border/50">
									{#if tool.input}
										<div class="mt-2">
											<div class="text-xs text-text-muted mb-1">Input</div>
											<pre class="text-xs bg-surface-900 rounded p-2 overflow-x-auto font-mono text-text-secondary max-h-96">{JSON.stringify(tool.input, null, 2)}</pre>
										</div>
									{/if}
									{#if tool.result !== undefined}
										<div class="mt-2">
											<div class="text-xs text-text-muted mb-1">Result</div>
											<pre class="text-xs bg-surface-900 rounded p-2 overflow-x-auto font-mono text-text-secondary max-h-96">{formatContent(tool.result)}</pre>
										</div>
									{/if}
								</div>
							{/if}
						</div>
					{/each}
				</div>
			{/if}
		{/each}
	</div>
{/if}
