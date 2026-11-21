'use client';

import { useEffect, useRef, useState } from 'react';
import Script from 'next/script';

import { swaggerAccessibility } from '@/components/docs/swagger-accessibility';
import { Button } from '@/components/ui/button';

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
  const [requested, setRequested] = useState(false);
  const [scriptReady, setScriptReady] = useState(false);
  const [stylesReady, setStylesReady] = useState(false);
  const [initialized, setInitialized] = useState(false);
  const [error, setError] = useState('');
  const trigger = useRef<HTMLButtonElement>(null);
  const explorer = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (window.location.hash.startsWith('#/')) setRequested(true);
  }, []);

  useEffect(() => {
    if (!scriptReady || !stylesReady || !explorer.current) return;

    const bundle = (window as WindowWithSwagger).SwaggerUIBundle;
    if (!bundle) {
      setError('Swagger UI failed to load in the browser.');
      return;
    }

    try {
      bundle({
        url: specURL,
        domNode: explorer.current,
        deepLinking: true,
        docExpansion: 'list',
        defaultModelsExpandDepth: 1,
        defaultModelExpandDepth: 1,
        displayRequestDuration: true,
        filter: true,
        plugins: [swaggerAccessibility],
        layout: 'BaseLayout',
      });
      if (trigger.current === document.activeElement) explorer.current.focus();
      setInitialized(true);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'Swagger UI initialization failed.');
    }
  }, [scriptReady, stylesReady, specURL]);

  const showAssetError = () => setError('Swagger UI assets could not be loaded from the CDN.');

  return (
    <div className="space-y-3">
      {!initialized && (
        <Button asChild>
          <button
            ref={trigger}
            type="button"
            aria-disabled={requested || undefined}
            onClick={() => setRequested(true)}
          >
            Load API explorer
          </button>
        </Button>
      )}
      {requested && !initialized && !error && (
        <p role="status" className="text-sm text-muted-foreground">Loading API explorer…</p>
      )}
      {error && (
        <p role="alert" data-slot="well" className="rounded-sm border px-4 py-3 text-sm text-muted-foreground">
          {error} Use the raw YAML link above.
        </p>
      )}
      {requested && (
        <>
          <link
            rel="stylesheet"
            href={`${SWAGGER_ASSETS}/swagger-ui.css`}
            integrity={SWAGGER_CSS_INTEGRITY}
            crossOrigin="anonymous"
            onLoad={() => setStylesReady(true)}
            onError={showAssetError}
          />
          <Script
            src={`${SWAGGER_ASSETS}/swagger-ui-bundle.js`}
            integrity={SWAGGER_BUNDLE_INTEGRITY}
            crossOrigin="anonymous"
            strategy="afterInteractive"
            onReady={() => setScriptReady(true)}
            onError={showAssetError}
          />
          <div data-slot={initialized ? 'well' : undefined} className={initialized ? 'rounded-sm border bg-card p-4' : undefined}>
            <div
              ref={explorer}
              role="region"
              aria-label="API explorer"
              tabIndex={-1}
              className="swagger-shell"
            />
          </div>
        </>
      )}
    </div>
  );
}
