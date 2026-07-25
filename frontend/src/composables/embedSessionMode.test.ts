import assert from 'node:assert/strict'
import test from 'node:test'

import { resolveSessionMode, shouldPersistMultiSession } from './embedSessionMode'

test('resolveSessionMode defaults web-link routes to multi', () => {
  assert.equal(resolveSessionMode(true), 'multi')
})

test('resolveSessionMode defaults non-web-link routes (iframe/widget) to single_fresh', () => {
  assert.equal(resolveSessionMode(false), 'single_fresh')
})

test('resolveSessionMode lets an explicit override win over a web-link route', () => {
  assert.equal(resolveSessionMode(true, 'single_fresh'), 'single_fresh')
})

test('resolveSessionMode lets an explicit override win over a non-web-link route', () => {
  assert.equal(resolveSessionMode(false, 'multi'), 'multi')
})

test('shouldPersistMultiSession is true only for multi sessions', () => {
  assert.equal(shouldPersistMultiSession('multi'), true)
  assert.equal(shouldPersistMultiSession('single_fresh'), false)
})
