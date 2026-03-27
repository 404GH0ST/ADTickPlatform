import { readParticipantOpenAPISpec } from '@/lib/repo-docs';

export async function GET() {
  try {
    const body = await readParticipantOpenAPISpec();
    return new Response(body, {
      status: 200,
      headers: {
        'content-type': 'application/yaml; charset=utf-8',
        'content-disposition': 'inline; filename="platform-api-v2.openapi.yaml"',
        'cache-control': 'public, max-age=300',
      },
    });
  } catch {
    return new Response('OpenAPI spec is unavailable.\n', {
      status: 404,
      headers: { 'content-type': 'text/plain; charset=utf-8' },
    });
  }
}
