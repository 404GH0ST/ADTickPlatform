import type {
  ComponentType, MouseEvent, ReactNode, SyntheticEvent, createContext,
  createElement, Fragment, useContext, useId, useLayoutEffect, useRef,
} from 'react';

type SwaggerComponent = ComponentType<Record<string, unknown>>;
type ComponentLookup = (name: string, container?: boolean) => SwaggerComponent;
type Operation = {
  tag: string;
  operationId: string;
  method: string;
  path: string;
  isDeepLinkingEnabled: boolean;
};
type OperationSummaryProps = {
  getComponent: ComponentLookup;
  operationProps: { toObject(): Operation };
  isShown: boolean;
  toggleShown: () => void;
};
type AuthorizationPopupProps = {
  authSelectors: { shownDefinitions(): { valueSeq(): { toArray(): unknown[] } } };
  authActions: { showDefinitions(show: false): void };
  getComponent: ComponentLookup;
  errSelectors: unknown;
  specSelectors: unknown;
  fn: { AST?: unknown };
};

export function swaggerAccessibility({ React }: {
  React: {
    createContext: typeof createContext;
    createElement: typeof createElement;
    Fragment: typeof Fragment;
    useContext: typeof useContext;
    useId: typeof useId;
    useLayoutEffect: typeof useLayoutEffect;
    useRef: typeof useRef;
  };
}) {
  // Swagger renders with its bundled React runtime.
  const { createElement: element, Fragment: fragment } = React;
  const PathText: SwaggerComponent = ({ text }) => element('span', null, text as ReactNode);
  const OperationContext = React.createContext<{
    CopyButton: SwaggerComponent;
    operation: Operation;
    isShown: boolean;
    toggleShown: () => void;
  } | null>(null);

  function OperationLinks(copyProps: Record<string, unknown>) {
    const context = React.useContext(OperationContext);
    if (!context) return null;

    const { CopyButton, operation, isShown, toggleShown } = context;
    const hash = [operation.tag, operation.operationId]
      .map((part) => encodeURIComponent(part.trim())).join('/');
    return element(fragment, null,
      element(CopyButton, copyProps),
      element('a', {
        className: 'operation-permalink',
        ref: (link: HTMLAnchorElement | null) => {
          // Swagger names both its generic wrapper and the real copy button.
          const wrapper = link?.previousElementSibling;
          if (wrapper?.classList.contains('copy-to-clipboard')) {
            wrapper.removeAttribute('aria-label');
          }
        },
        href: `#/${hash}`,
        'aria-label': `Link to ${operation.method.toUpperCase()} ${operation.path}`,
        onClick: (event: MouseEvent<HTMLAnchorElement>) => {
          if (isShown || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
          event.preventDefault();
          toggleShown();
        },
      }, 'Link'),
    );
  }

  function AuthorizationDialog({
    authSelectors, authActions, getComponent, errSelectors, specSelectors, fn,
  }: AuthorizationPopupProps) {
    const dialogRef = React.useRef<HTMLDialogElement>(null);
    const headingId = React.useId();
    const AuthorizationForm = getComponent('auths');
    const CloseIcon = getComponent('CloseIcon');
    const close = () => authActions.showDefinitions(false);

    React.useLayoutEffect(() => {
      const dialog = dialogRef.current;
      if (!dialog) return;

      dialog.showModal();
      const input = dialog.querySelector<HTMLInputElement>('input:not(:disabled)');
      if (input && input.getBoundingClientRect().bottom <= dialog.getBoundingClientRect().bottom) {
        input.focus({ preventScroll: true });
      }
      return () => dialog.close();
    }, []);

    return element('div', { className: 'dialog-ux' },
      element('dialog', {
        ref: dialogRef,
        className: 'modal-ux',
        'aria-labelledby': headingId,
        onCancel: (event: SyntheticEvent<HTMLDialogElement>) => {
          event.preventDefault();
          close();
        },
        onClick: (event: MouseEvent<HTMLDialogElement>) => {
          if (event.target !== event.currentTarget) return;
          const bounds = event.currentTarget.getBoundingClientRect();
          const outside = event.clientX < bounds.left || event.clientX > bounds.right ||
            event.clientY < bounds.top || event.clientY > bounds.bottom;
          if (outside) close();
        },
      },
        element('div', { className: 'modal-ux-header' },
          element('button', {
            type: 'button',
            className: 'close-modal',
            'aria-label': 'Close authorization',
            onClick: close,
          }, element(CloseIcon)),
          element('h3', { id: headingId }, 'Available authorizations'),
        ),
        element('div', { className: 'modal-ux-content' },
          authSelectors.shownDefinitions().valueSeq().toArray().map((definition, index) =>
            element(AuthorizationForm, {
              key: index,
              AST: fn.AST ?? {},
              definitions: definition,
              getComponent,
              errSelectors,
              authSelectors,
              authActions,
              specSelectors,
            }),
          ),
        ),
      ),
    );
  }

  return {
    wrapComponents: {
      authorizationPopup: () => AuthorizationDialog,
      Button: (Original: SwaggerComponent) =>
        function Button(props: Record<string, unknown>) {
          // The visible form labels are the names used by voice controls.
          const label = props['aria-label'];
          return label === 'Apply credentials' || label === 'Remove authorization'
            ? element(Original, { ...props, 'aria-label': undefined })
            : element(Original, props);
        },
      Input: (Original: SwaggerComponent) =>
        function Input(props: Record<string, unknown>) {
          // Use Swagger's associated Value label in place of its element ID.
          return props.id === 'auth-bearer-value' && props['aria-label'] === props.id
            ? element(Original, { ...props, 'aria-label': undefined })
            : element(Original, props);
        },
      TextArea: (Original: SwaggerComponent) =>
        function TextArea(props: Record<string, unknown>) {
          const id = React.useId();
          if (typeof props.className !== 'string' || !props.className.split(' ').includes('body-param__text')) {
            return element(Original, props);
          }
          return element(fragment, null,
            element('label', { className: 'sr-only', htmlFor: id }, 'Request body'),
            element(Original, { ...props, id }),
          );
        },
      OperationSummaryPath: (Original: ComponentType<OperationSummaryProps>) =>
        function OperationSummaryPath(props: OperationSummaryProps) {
          return element(Original, {
            ...props,
            getComponent: (name, container) => name === 'DeepLink'
              ? PathText
              : props.getComponent(name, container),
          });
        },
      OperationSummary: (Original: ComponentType<OperationSummaryProps>) =>
        function OperationSummary(props: OperationSummaryProps) {
          const operation = props.operationProps.toObject();
          if (!operation.isDeepLinkingEnabled) return element(Original, props);

          return element(OperationContext.Provider, {
            value: {
              CopyButton: props.getComponent('CopyToClipboardBtn', true),
              operation,
              isShown: props.isShown,
              toggleShown: props.toggleShown,
            },
          }, element(Original, {
            ...props,
            getComponent: (name, container) => name === 'CopyToClipboardBtn'
              ? OperationLinks
              : props.getComponent(name, container),
          }));
        },
      Servers: (Original: SwaggerComponent) =>
        function Servers(props: Record<string, unknown>) {
          return element('div', {
            className: 'contents',
            ref: (wrapper: HTMLDivElement | null) => {
              wrapper?.querySelector('select#servers')?.setAttribute('aria-label', 'Server');
            },
          }, element(Original, props));
        },
      ExamplesSelect: (Original: SwaggerComponent) =>
        function ExamplesSelect(props: Record<string, unknown>) {
          const id = React.useId();
          return element('div', {
            className: 'contents',
            ref: (wrapper: HTMLDivElement | null) => {
              const select = wrapper?.querySelector('select');
              if (select) select.id = id;
            },
          },
          element('label', { className: 'sr-only', htmlFor: id }, 'Examples'),
          element(Original, props));
        },
    },
  };
}
