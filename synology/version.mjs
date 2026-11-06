#!/usr/bin/env node
// Stable tag -> DSM version. Keep all wrapper-owned changes under synology/.
import {readFileSync, writeFileSync} from 'node:fs';
const [tag, build, mode] = process.argv.slice(2);
const match = /^v?((?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*))$/.exec(tag ?? '');
const file = new URL('./dsm-package.yaml', import.meta.url);
const source = readFileSync(file, 'utf8');
const current = /^build: (\d+)\s*$/m.exec(source);
const currentVersion = /^version: (.+?)\s*$/m.exec(source)?.[1];
// Our scheme is <upstream version>-<build>: the build restarts at 1 with every
// new upstream version and only has to increase within one version.
const sameVersion = match && currentVersion === match[1];
if (!match || !/^[1-9][0-9]{0,3}$/.test(build ?? '') || !current || (sameVersion && Number(build) <= Number(current[1])))
  throw new Error('Use a stable [v]X.Y.Z tag and a packaging build (1–9999) that increases within one upstream version.');
if (!/^version: .+$/m.test(source)) throw new Error('Missing scaffold version.');
if (mode !== '--validate') writeFileSync(file, source.replace(/^version: .+$/m, `version: ${match[1]}`).replace(/^build: \d+\s*$/m, `build: ${Number(build)}`));
