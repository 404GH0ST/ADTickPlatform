'use client';

import { useEffect, useMemo, useState } from 'react';
import Script from 'next/script';

import { swaggerAccessibility } from '@/components/docs/swagger-accessibility';

// The accessibility plugin wraps components from this Swagger UI release.
const SWAGGER_ASSETS = 'https://unpkg.com/swagger-ui-dist@5.35.0';
// Subresource Integrity pins the exact third-party bytes. The docs page shares
// an origin with the organizer dashboard, so a tampered CDN response must fail
// to load rather than run with same-origin access. Recompute both hashes when
// bumping the version above (sha384 of each file, base64).
const SWAGGER_BUNDLE_INTEGRITY = 'sha384-gjRm0bEPMeTmMh5jT2cHccIwCUZSTd8YC+j0d0sXw0MdGEfScduGpSyF3ZSSLkX9';
const SWAGGER_CSS_INTEGRITY = 'sha384-WlYgdmXYOOBWL5apwjDEZwN+U3HmXSSu5R6V5aGxEcf7z+KuRNwBhoxKVN1i3D5I';

type SwaggerBundle = (config: Record<string, unknown>) => unknown;

type WindowWithSwagger = Window & {
  SwaggerUIBundle?: SwaggerBundle;
};

type Props = {
  specURL: string;
};

export function SwaggerUIBrowser({ specURL }: Props) {
  const [ready, setReady] = useState(false);
  const [error, setError] = useState('');

  const domID = useMemo(() => '#participant-swagger-ui', []);

  useEffect(() => {
    if (!ready) {
      return;
    }

    const bundle = (window as WindowWithSwagger).SwaggerUIBundle;
    if (!bundle) {
      setError('Swagger UI failed to load in the browser.');
      return;
    }

    try {
      bundle({
        url: specURL,
        dom_id: domID,
        deepLinking: true,
        docExpansion: 'list',
        defaultModelsExpandDepth: 1,
        defaultModelExpandDepth: 1,
        displayRequestDuration: true,
        filter: true,
        plugins: [swaggerAccessibility],
        layout: 'BaseLayout',
      });
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'Swagger UI initialization failed.');
    }
  }, [domID, ready, specURL]);

  return (
    <>
      <link
        rel="stylesheet"
        href={`${SWAGGER_ASSETS}/swagger-ui.css`}
        integrity={SWAGGER_CSS_INTEGRITY}
        crossOrigin="anonymous"
      />
      <Script
        src={`${SWAGGER_ASSETS}/swagger-ui-bundle.js`}
        integrity={SWAGGER_BUNDLE_INTEGRITY}
        crossOrigin="anonymous"
        strategy="afterInteractive"
        onLoad={() => setReady(true)}
        onError={() => setError('Swagger UI assets could not be loaded from the CDN.')}
      />

      {error ? (
        <div className="rounded-sm border px-4 py-3 text-sm text-muted-foreground">{error}</div>
      ) : null}

      <div className="rounded-sm border bg-card p-4">
        <div id="participant-swagger-ui" className="swagger-shell min-h-[44rem]" />
      </div>
    </>
  );
}
