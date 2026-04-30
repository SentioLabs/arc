// @vitest-environment jsdom
import { describe, it, expect, beforeEach } from 'vitest';
import { getReviewerName, setReviewerName, clearReviewerName } from './identity';

describe('identity', () => {
	beforeEach(() => localStorage.clear());

	it('round-trips a name', () => {
		setReviewerName('Alice');
		expect(getReviewerName()).toBe('Alice');
	});

	it('clear removes the name', () => {
		setReviewerName('Bob');
		clearReviewerName();
		expect(getReviewerName()).toBeNull();
	});

	it('trims whitespace on set', () => {
		setReviewerName('  Carol  ');
		expect(getReviewerName()).toBe('Carol');
	});
});
