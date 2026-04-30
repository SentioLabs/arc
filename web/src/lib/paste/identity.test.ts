// @vitest-environment jsdom
import { describe, it, expect, beforeEach } from 'vitest';
import { getReviewerName, setReviewerName, clearReviewerName, parseShareFragment } from './identity';

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

describe('parseShareFragment', () => {
	it('parses k only', () => {
		expect(parseShareFragment('#k=abc')).toEqual({ k: 'abc', t: null });
	});

	it('parses k and t', () => {
		expect(parseShareFragment('#k=abc&t=xyz')).toEqual({ k: 'abc', t: 'xyz' });
	});

	it('order-independent', () => {
		expect(parseShareFragment('#t=xyz&k=abc')).toEqual({ k: 'abc', t: 'xyz' });
	});

	it('handles missing leading #', () => {
		expect(parseShareFragment('k=abc&t=xyz')).toEqual({ k: 'abc', t: 'xyz' });
	});

	it('returns null for missing keys', () => {
		expect(parseShareFragment('')).toEqual({ k: null, t: null });
		expect(parseShareFragment('#')).toEqual({ k: null, t: null });
		expect(parseShareFragment('#other=foo')).toEqual({ k: null, t: null });
	});

	it('treats empty values as null', () => {
		expect(parseShareFragment('#k=&t=')).toEqual({ k: null, t: null });
	});
});
