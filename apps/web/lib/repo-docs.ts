import { readFile } from 'node:fs/promises';
import path from 'node:path';

function candidateDocsRoots(): string[] {
  return [
    path.resolve(process.cwd(), '..', '..', 'docs'),
    path.resolve(process.cwd(), 'docs'),
  ];
}

async function readRepoDocFile(filename: string): Promise<string> {
  for (const root of candidateDocsRoots()) {
    try {
      return await readFile(path.join(root, filename), 'utf8');
    } catch {
      continue;
    }
  }
  throw new Error(`repo doc ${filename} is unavailable`);
}

export async function readPlatformManual(): Promise<string> {
  return readRepoDocFile('platform-manual.md');
}

export async function readParticipantOpenAPISpec(): Promise<string> {
  return readRepoDocFile('platform-api-v2.openapi.yaml');
}

export function extractOpenAPIPathBlock(document: string, pathKey: string): string {
  const lines = document.split('\n');
  const marker = `  ${pathKey}:`;
  const startIndex = lines.findIndex((line) => line === marker);
  if (startIndex === -1) {
    return '';
  }

  const result: string[] = [];
  for (let index = startIndex; index < lines.length; index += 1) {
    const line = lines[index];
    if (index > startIndex && (/^  \/[^\s]+:/.test(line) || line === 'components:')) {
      break;
    }
    result.push(line);
  }
  return result.join('\n').trim();
}
