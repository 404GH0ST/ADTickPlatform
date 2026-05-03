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

export async function readParticipantOpenAPISpec(): Promise<string> {
  return readRepoDocFile('platform-api-v2.openapi.yaml');
}
