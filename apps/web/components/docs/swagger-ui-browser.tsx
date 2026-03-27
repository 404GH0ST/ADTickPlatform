'use client';

import { useEffect, useMemo, useState } from 'react';
import Script from 'next/script';

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
        layout: 'BaseLayout',
      });
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'Swagger UI initialization failed.');
    }
  }, [domID, ready, specURL]);

  return (
    <>
      <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css" />
      <Script
        src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"
        strategy="afterInteractive"
        onLoad={() => setReady(true)}
        onError={() => setError('Swagger UI assets could not be loaded from the CDN.')}
      />

      {error ? (
        <div className="rounded-lg border px-4 py-3 text-sm text-muted-foreground">{error}</div>
      ) : null}

      <div className="rounded-lg border bg-card p-4">
        <div id="participant-swagger-ui" className="swagger-shell min-h-[44rem]" />
      </div>
    </>
  );
}
