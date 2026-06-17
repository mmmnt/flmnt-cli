import { defineConfig } from 'vitest/config';

export default defineConfig({
	test: {
		globals: false,
		environment: 'node',
		passWithNoTests: true,
		reporters: process.env.CI
			? [['junit', { outputFile: 'junit-ts.xml', suiteName: 'QUORUM' }], 'default']
			: ['default'],
		coverage: {
			provider: 'v8',
			reporter: ['text', 'json-summary', 'lcov'],
			include: ['src/**'],
			all: true,
			thresholds: {
				lines: 95,
				branches: 95,
				functions: 95,
				statements: 95,
			},
		},
	},
});
