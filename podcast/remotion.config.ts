/**
 * Note: When using the Node.JS APIs, the config file
 * doesn't apply. Instead, pass options directly to the APIs.
 *
 * All configuration options: https://remotion.dev/docs/config
 */

import { Config } from '@remotion/cli/config';
import fs from 'node:fs';

Config.setVideoImageFormat('jpeg');
Config.setOverwriteOutput(true);

// Prefer a system-installed Chromium/Chrome if present.
// This avoids Remotion downloading a bundled browser that may fail to launch on minimal servers
// due to missing shared libraries.
const envExecutable =
	process.env.AI_THINGS_BROWSER_EXECUTABLE ??
	process.env.REMOTION_BROWSER_EXECUTABLE ??
	process.env.PUPPETEER_EXECUTABLE_PATH ??
	process.env.CHROME_PATH ??
	null;

const candidates = [
	envExecutable,
	'/usr/bin/chromium',
	'/usr/bin/chromium-browser',
	'/usr/bin/google-chrome',
	'/usr/bin/google-chrome-stable',
].filter(Boolean) as string[];

const found = candidates.find((p) => fs.existsSync(p));
if (found) {
	Config.setBrowserExecutable(found);
}

// This template processes the whole audio file on each thread which is heavy.
// You are safe to increase concurrency if the audio file is small or your machine strong!
Config.setConcurrency(1);
