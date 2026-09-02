// @ts-nocheck
import { describe, expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

const componentSource = readFileSync(resolve(import.meta.dir, 'TypeBadge.svelte'), 'utf-8');

describe('TypeBadge component', () => {
	test('defines a release entry in typeConfig with a distinct color', () => {
		expect(componentSource).toMatch(/release:\s*{[^}]*color:\s*'text-type-release'/s);
	});

	test('defines a milestone entry in typeConfig with a distinct color', () => {
		expect(componentSource).toMatch(/milestone:\s*{[^}]*color:\s*'text-type-milestone'/s);
	});

	test('release and milestone use different colors from each other and from epic', () => {
		const epicMatch = componentSource.match(/epic:\s*{[^}]*color:\s*'([^']+)'/s);
		const releaseMatch = componentSource.match(/release:\s*{[^}]*color:\s*'([^']+)'/s);
		const milestoneMatch = componentSource.match(/milestone:\s*{[^}]*color:\s*'([^']+)'/s);

		expect(epicMatch).not.toBeNull();
		expect(releaseMatch).not.toBeNull();
		expect(milestoneMatch).not.toBeNull();

		const colors = new Set([epicMatch[1], releaseMatch[1], milestoneMatch[1]]);
		expect(colors.size).toBe(3);
	});
});
