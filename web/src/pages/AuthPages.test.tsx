/**
 * Tests for LoginPage, SetupPage, and SettingsGeneralPage.
 *
 * All auth page tests live in one file so they share a single MSW server
 * instance, avoiding concurrent SQLite cookie-store contention that occurs
 * when multiple test files each call setupServer().listen() in parallel.
 */
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, beforeAll, afterEach, afterAll, vi } from 'vitest'
import { http, HttpResponse } from 'msw'
import { setupServer } from 'msw/node'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { LoginPage } from './LoginPage'
import { SetupPage } from './SetupPage'
import { SettingsGeneralPage } from './SettingsGeneralPage'

// ── MSW server ─────────────────────────────────────────────────────────────────

const server = setupServer(
  http.get('/api/auth/me', () => HttpResponse.json({ username: 'testuser' })),
)

beforeAll(() => server.listen({ onUnhandledRequest: 'error' }))
afterEach(() => { server.resetHandlers(); vi.restoreAllMocks(); vi.unstubAllGlobals() })
afterAll(() => server.close())

// ── helpers ────────────────────────────────────────────────────────────────────

/**
 * Intercept window.location.href assignments on the real Location object so
 * they don't cause happy-dom to navigate (which unmounts the component and
 * breaks assertions).  Returns a spy that records all values assigned to href.
 *
 * We shadow `href` directly on window.location rather than replacing the entire
 * Location object so that MSW can still read window.location.origin/host/etc.
 * when it intercepts fetch requests.
 */
function mockHref(): ReturnType<typeof vi.fn> {
  const hrefSetter = vi.fn()
  // Initialize to the real current URL so that happy-dom can still resolve
  // relative fetch URLs (e.g. '/api/auth/login') against a valid base.
  let _href = window.location.href
  Object.defineProperty(window.location, 'href', {
    configurable: true,
    get: () => _href,
    set: (v: string) => { _href = v; hrefSetter(v) },
  })
  return hrefSetter
}

/**
 * Override window.location.protocol on the real Location object.
 * Default in happy-dom is 'http:'; call this with 'https:' to simulate HTTPS.
 */
function setProtocol(protocol: string) {
  Object.defineProperty(window.location, 'protocol', {
    configurable: true,
    get: () => protocol,
  })
}

afterEach(() => {
  // Restore any own-property overrides placed on window.location.
  for (const prop of ['href', 'protocol'] as const) {
    if (Object.prototype.hasOwnProperty.call(window.location, prop)) {
      try { delete (window.location as unknown as Record<string, unknown>)[prop] } catch { /* ignore */ }
    }
  }
})

// ── render helpers ─────────────────────────────────────────────────────────────

function renderLogin() {
  const { container } = render(<LoginPage />)
  return {
    usernameInput: container.querySelector<HTMLInputElement>('input[autocomplete="username"]')!,
    passwordInput: container.querySelector<HTMLInputElement>('input[autocomplete="current-password"]')!,
  }
}

function renderSetup() {
  const { container } = render(<SetupPage />)
  const passwordInputs = container.querySelectorAll<HTMLInputElement>('input[autocomplete="new-password"]')
  return {
    usernameInput: container.querySelector<HTMLInputElement>('input[autocomplete="username"]')!,
    passwordInput: passwordInputs[0],
    confirmInput: passwordInputs[1],
  }
}

function renderSettingsGeneral() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  const { container } = render(
    <QueryClientProvider client={qc}>
      <SettingsGeneralPage />
    </QueryClientProvider>,
  )
  const newPasswordInputs = container.querySelectorAll<HTMLInputElement>('input[autocomplete="new-password"]')
  return {
    newUsernameInput: container.querySelector<HTMLInputElement>('input[autocomplete="username"]')!,
    newPasswordInput: newPasswordInputs[0],
    confirmPasswordInput: newPasswordInputs[1],
    currentPasswordInput: container.querySelector<HTMLInputElement>('input[autocomplete="current-password"]')!,
  }
}

// ── LoginPage ─────────────────────────────────────────────────────────────────

describe('LoginPage', () => {
  it('renders username, password fields and sign in button', () => {
    const { usernameInput, passwordInput } = renderLogin()
    expect(usernameInput).toBeInTheDocument()
    expect(passwordInput).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /sign in/i })).toBeInTheDocument()
  })

  it('redirects to /jobs on successful login', async () => {
    server.use(http.post('/api/auth/login', () => new HttpResponse(null, { status: 204 })))
    const hrefSetter = mockHref()

    const { usernameInput, passwordInput } = renderLogin()
    await userEvent.type(usernameInput, 'admin')
    await userEvent.type(passwordInput, 'secret')
    await userEvent.click(screen.getByRole('button', { name: /sign in/i }))

    await waitFor(() => expect(hrefSetter).toHaveBeenCalledWith('/jobs'))
  })

  it('shows error message on failed login', async () => {
    server.use(
      http.post('/api/auth/login', () =>
        HttpResponse.json({ error: 'invalid credentials' }, { status: 400 }),
      ),
    )

    const { usernameInput, passwordInput } = renderLogin()
    await userEvent.type(usernameInput, 'admin')
    await userEvent.type(passwordInput, 'wrong')
    await userEvent.click(screen.getByRole('button', { name: /sign in/i }))

    await waitFor(() => expect(screen.getByText(/invalid credentials/i)).toBeInTheDocument())
  })

  it('disables submit button while loading', async () => {
    let resolveRequest!: () => void
    const blocker = new Promise<void>(r => { resolveRequest = r })
    server.use(
      http.post('/api/auth/login', async () => {
        await blocker
        return new HttpResponse(null, { status: 204 })
      }),
    )
    mockHref()

    const { usernameInput, passwordInput } = renderLogin()
    await userEvent.type(usernameInput, 'admin')
    await userEvent.type(passwordInput, 'secret')

    const clickPromise = userEvent.click(screen.getByRole('button', { name: /sign in/i }))

    await waitFor(() =>
      expect(screen.getByRole('button', { name: /signing in/i })).toBeDisabled(),
    )
    resolveRequest()
    await clickPromise
  })

  it('shows insecure connection warning when protocol is http', () => {
    // happy-dom defaults to http: so no setup needed.
    renderLogin()
    expect(screen.getByText(/insecure connection/i)).toBeInTheDocument()
  })

  it('does not show insecure warning when protocol is https', () => {
    setProtocol('https:')
    renderLogin()
    expect(screen.queryByText(/insecure connection/i)).not.toBeInTheDocument()
  })
})

// ── SetupPage ─────────────────────────────────────────────────────────────────

describe('SetupPage', () => {
  it('renders username, password, confirm fields and create account button', () => {
    const { usernameInput, passwordInput, confirmInput } = renderSetup()
    expect(usernameInput).toBeInTheDocument()
    expect(passwordInput).toBeInTheDocument()
    expect(confirmInput).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /create account/i })).toBeInTheDocument()
  })

  it('shows error when passwords do not match without making API call', async () => {
    const { usernameInput, passwordInput, confirmInput } = renderSetup()
    await userEvent.type(usernameInput, 'admin')
    await userEvent.type(passwordInput, 'secret')
    await userEvent.type(confirmInput, 'different')
    await userEvent.click(screen.getByRole('button', { name: /create account/i }))
    expect(screen.getByText(/passwords do not match/i)).toBeInTheDocument()
  })

  it('redirects to /login on successful setup', async () => {
    server.use(http.post('/api/auth/setup', () => new HttpResponse(null, { status: 204 })))
    const hrefSetter = mockHref()

    const { usernameInput, passwordInput, confirmInput } = renderSetup()
    await userEvent.type(usernameInput, 'admin')
    await userEvent.type(passwordInput, 'secret')
    await userEvent.type(confirmInput, 'secret')
    await userEvent.click(screen.getByRole('button', { name: /create account/i }))

    await waitFor(() => expect(hrefSetter).toHaveBeenCalledWith('/login'))
  })

  it('shows API error message on setup failure', async () => {
    server.use(
      http.post('/api/auth/setup', () =>
        HttpResponse.json({ error: 'already configured' }, { status: 409 }),
      ),
    )

    const { usernameInput, passwordInput, confirmInput } = renderSetup()
    await userEvent.type(usernameInput, 'admin')
    await userEvent.type(passwordInput, 'secret')
    await userEvent.type(confirmInput, 'secret')
    await userEvent.click(screen.getByRole('button', { name: /create account/i }))

    await waitFor(() => expect(screen.getByText(/already configured/i)).toBeInTheDocument())
  })

  it('shows insecure connection warning when protocol is http', () => {
    renderSetup()
    expect(screen.getByText(/insecure connection/i)).toBeInTheDocument()
  })
})

// ── SettingsGeneralPage ────────────────────────────────────────────────────────

describe('SettingsGeneralPage', () => {
  it('pre-fills username placeholder from /me response', async () => {
    renderSettingsGeneral()
    await waitFor(() =>
      expect(screen.getByPlaceholderText('testuser')).toBeInTheDocument(),
    )
  })

  it('renders all credential fields and save button', () => {
    const { newUsernameInput, newPasswordInput, confirmPasswordInput, currentPasswordInput } =
      renderSettingsGeneral()
    expect(newUsernameInput).toBeInTheDocument()
    expect(newPasswordInput).toBeInTheDocument()
    expect(confirmPasswordInput).toBeInTheDocument()
    expect(currentPasswordInput).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /save changes/i })).toBeInTheDocument()
  })

  it('shows error when new passwords do not match without making API call', async () => {
    const { newPasswordInput, confirmPasswordInput, currentPasswordInput } = renderSettingsGeneral()
    await userEvent.type(newPasswordInput, 'newpass')
    await userEvent.type(confirmPasswordInput, 'different')
    await userEvent.type(currentPasswordInput, 'testpass')
    await userEvent.click(screen.getByRole('button', { name: /save changes/i }))
    expect(screen.getByText(/passwords do not match/i)).toBeInTheDocument()
  })

  it('shows success message and redirects to login after credential update', async () => {
    server.use(
      http.put('/api/auth/credentials', () => new HttpResponse(null, { status: 204 })),
    )
    const hrefSetter = mockHref()

    const { newPasswordInput, confirmPasswordInput, currentPasswordInput } = renderSettingsGeneral()
    await userEvent.type(newPasswordInput, 'newpass')
    await userEvent.type(confirmPasswordInput, 'newpass')
    await userEvent.type(currentPasswordInput, 'testpass')
    await userEvent.click(screen.getByRole('button', { name: /save changes/i }))

    await waitFor(() =>
      expect(screen.getByText(/credentials updated/i)).toBeInTheDocument(),
    )
    // The component redirects to /login after a 1500ms timeout.
    await waitFor(() => expect(hrefSetter).toHaveBeenCalledWith('/login'), { timeout: 3000 })
  })

  it('disables button while saving', async () => {
    let resolveRequest!: () => void
    const blocker = new Promise<void>(r => { resolveRequest = r })
    server.use(
      http.put('/api/auth/credentials', async () => {
        await blocker
        return new HttpResponse(null, { status: 204 })
      }),
    )
    mockHref()

    const { currentPasswordInput } = renderSettingsGeneral()
    await userEvent.type(currentPasswordInput, 'testpass')

    const clickPromise = userEvent.click(screen.getByRole('button', { name: /save changes/i }))

    await waitFor(() =>
      expect(screen.getByRole('button', { name: /saving/i })).toBeDisabled(),
    )
    resolveRequest()
    await clickPromise
  })
})
