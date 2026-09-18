// @ts-nocheck
import { describe, expect, test } from 'vitest';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

const utilsSource = readFileSync(resolve(import.meta.dirname, 'utils.ts'), 'utf-8');

describe('issueTypeLabels', () => {
	test('labels release as Release', () => {
		expect(utilsSource).toMatch(/release:\s*'Release'/);
	});

	test('labels milestone as Milestone', () => {
		expect(utilsSource).toMatch(/milestone:\s*'Milestone'/);
	});
});
