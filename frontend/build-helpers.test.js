// Run with: npm test   (Node's built-in test runner, no extra dependency)
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { normalizeApiBase, contentSecurityPolicy } from './build-helpers.js';

test('VITE_API_BASE: empty or unset means same origin', () => {
  assert.equal(normalizeApiBase(undefined), '');
  assert.equal(normalizeApiBase('   '), '');
});

test('VITE_API_BASE: an origin is accepted and normalised', () => {
  assert.equal(normalizeApiBase('https://api.example.com/'), 'https://api.example.com');
  assert.equal(normalizeApiBase(' https://API.example.com:8443 '), 'https://api.example.com:8443');
  assert.equal(normalizeApiBase('http://localhost:8080'), 'http://localhost:8080');
});

test('VITE_API_BASE: anything else stops the build', () => {
  for (const bad of ['api.example.com', 'https://api.example.com/api/v1', 'ftp://api.example.com', 'https://api.example.com?x=1', 'https://u:p@api.example.com']) {
    assert.throws(() => normalizeApiBase(bad), /VITE_API_BASE/, bad);
  }
});

test('CSP: opens connect-src for the API and nothing else', () => {
  const csp = contentSecurityPolicy('https://api.example.com');
  assert.match(csp, /connect-src 'self' https:\/\/api\.example\.com(;|$)/);
  assert.match(csp, /script-src 'self'/);
  assert.match(csp, /object-src 'none'/);
  assert.doesNotMatch(csp, /unsafe-inline|unsafe-eval|\*/);
});
