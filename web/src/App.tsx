import { useEffect, useMemo, useState } from 'react'
import {
  ActionIcon,
  Alert,
  AppShell,
  Badge,
  Box,
  Button,
  Container,
  Divider,
  Group,
  Loader,
  NavLink,
  Paper,
  ScrollArea,
  Select,
  Stack,
  Text,
  TextInput,
  Textarea,
  Title,
} from '@mantine/core'
import {
  BASE_URL,
  HISTORY_KINDS,
  UnauthorizedError,
  type Artifact,
  type Department,
  type Exception,
  type Health,
  type Job,
  clearToken,
  getArtifact,
  getHealth,
  getToken,
  listArtifactHistory,
  listDepartments,
  listExceptions,
  listJobs,
  resolveException,
  runDepartment,
  setToken,
  subscribeToExceptionChanges,
  subscribeToJobChanges,
} from './api'

const VIEW_DEPARTMENT = 'department'
const VIEW_EXCEPTIONS = 'exceptions'
const VIEW_HISTORY = 'history'
type View = typeof VIEW_DEPARTMENT | typeof VIEW_EXCEPTIONS | typeof VIEW_HISTORY

const CONSEQUENCE_COLOR: Record<string, string> = {
  R4: 'red',
  R3: 'orange',
  R2: 'yellow',
  R1: 'green',
  R0: 'gray',
}

const JOB_STATUS_COLOR: Record<string, string> = {
  pending: 'gray',
  claimed: 'blue',
  running: 'blue',
  done: 'green',
  failed: 'red',
  escalated: 'orange',
  quarantined: 'grape',
}

const RUNG_COLOR: Record<string, string> = {
  none: 'gray',
  desk: 'yellow',
  analogue: 'yellow',
  stated: 'orange',
  behaviour: 'green',
  commitment: 'green',
  revenue: 'green',
}

const LINK_STATUS_COLOR: Record<string, string> = {
  holds: 'green',
  breaks: 'red',
  unchecked: 'gray',
}

function humanizeKey(key: string): string {
  const spaced = key.replace(/([a-z])([A-Z])/g, '$1 $2')
  return spaced.charAt(0).toUpperCase() + spaced.slice(1)
}

function PredicateValue({ fieldKey, value }: { fieldKey: string; value: unknown }) {
  if (value === null || value === undefined || value === '') {
    return (
      <Text size="sm" c="dimmed">
        -
      </Text>
    )
  }
  if (fieldKey === 'status' && typeof value === 'string') {
    return <Badge color={LINK_STATUS_COLOR[value] ?? 'gray'}>{value}</Badge>
  }
  if (fieldKey === 'rung' && typeof value === 'string') {
    return <Badge color={RUNG_COLOR[value] ?? 'gray'}>{value}</Badge>
  }
  if (typeof value === 'string') {
    return (
      <Text size="sm" style={{ whiteSpace: 'pre-wrap' }}>
        {value}
      </Text>
    )
  }
  if (typeof value === 'boolean' || typeof value === 'number') {
    return <Text size="sm">{String(value)}</Text>
  }
  if (Array.isArray(value)) {
    if (value.length === 0) {
      return (
        <Text size="sm" c="dimmed">
          none
        </Text>
      )
    }
    if (value.every((v) => typeof v === 'string' || typeof v === 'number')) {
      return (
        <Stack gap={2}>
          {value.map((v, i) => (
            <Text size="sm" key={i}>
              - {String(v)}
            </Text>
          ))}
        </Stack>
      )
    }
    return (
      <Stack gap="xs">
        {value.map((item, i) => (
          <Paper key={i} withBorder p="xs" radius="sm">
            <PredicateFields obj={item as Record<string, unknown>} />
          </Paper>
        ))}
      </Stack>
    )
  }
  if (typeof value === 'object') {
    return <PredicateFields obj={value as Record<string, unknown>} />
  }
  return <Text size="sm">{String(value)}</Text>
}

function PredicateFields({ obj }: { obj: Record<string, unknown> }) {
  return (
    <Stack gap="xs">
      {Object.entries(obj).map(([key, value]) => (
        <div key={key}>
          <Text size="xs" fw={700} tt="uppercase" c="dimmed">
            {humanizeKey(key)}
          </Text>
          <PredicateValue fieldKey={key} value={value} />
        </div>
      ))}
    </Stack>
  )
}

function StatusPill({ health, connected }: { health: Health | null; connected: boolean }) {
  if (!connected) {
    return (
      <Group gap={6}>
        <Box w={8} h={8} bg="red" style={{ borderRadius: '50%' }} />
        <Text size="xs" c="red">
          disconnected
        </Text>
      </Group>
    )
  }
  if (!health) {
    return <Loader size="xs" />
  }
  return (
    <Group gap={6}>
      <Box w={8} h={8} bg="green" style={{ borderRadius: '50%' }} />
      <Text size="xs" c="dimmed">
        running since {new Date(health.startedAt).toLocaleTimeString()}
      </Text>
    </Group>
  )
}

function DisconnectedPanel() {
  return (
    <Container size="xs" pt="xl">
      <Stack align="center" gap="xs" mt="xl">
        <Title order={3} c="red">
          hr-server unreachable
        </Title>
        <Text c="dimmed" ta="center">
          Can't reach {BASE_URL}. Make sure <code>hr</code> is still running - checking again automatically.
        </Text>
        <Loader mt="md" />
      </Stack>
    </Container>
  )
}

function TokenGate({ onSaved }: { onSaved: () => void }) {
  const [value, setValue] = useState('')

  const save = () => {
    const trimmed = value.trim()
    if (!trimmed) return
    setToken(trimmed)
    onSaved()
  }

  return (
    <Container size="xs" pt="xl">
      <Stack align="center" gap="xs" mt="xl">
        <Title order={3}>Sign in to hr</Title>
        <Text c="dimmed" ta="center" size="sm">
          Paste the token from <code>hr identity create --name you</code>. It's stored only in this
          browser and sent as a bearer token on every request.
        </Text>
        <TextInput
          w={320}
          mt="sm"
          placeholder="hr_..."
          value={value}
          onChange={(e) => setValue(e.currentTarget.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter') save()
          }}
        />
        <Button onClick={save} disabled={!value.trim()}>
          Continue
        </Button>
      </Stack>
    </Container>
  )
}

function AssistantResult({ digest }: { digest: string }) {
  const [artifact, setArtifact] = useState<Artifact | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [expanded, setExpanded] = useState(false)

  useEffect(() => {
    getArtifact(digest)
      .then(setArtifact)
      .catch((e) => setError(e instanceof Error ? e.message : String(e)))
  }, [digest])

  if (error) return <Text size="sm" c="red">{error}</Text>
  if (!artifact) return <Loader size="xs" />

  const preview =
    typeof artifact.predicate.claim === 'string'
      ? artifact.predicate.claim
      : typeof artifact.predicate.because === 'string'
        ? artifact.predicate.because
        : null

  return (
    <Stack gap={4}>
      <Group gap="xs" wrap="nowrap" align="flex-start">
        <Badge variant="light" size="sm">
          {artifact.kind}
        </Badge>
        {preview && !expanded && (
          <Text size="sm" c="dimmed" lineClamp={1}>
            {preview}
          </Text>
        )}
      </Group>
      <Button variant="subtle" size="compact-xs" w="fit-content" px={0} onClick={() => setExpanded((v) => !v)}>
        {expanded ? 'Hide details' : 'View details'}
      </Button>
      {expanded && <PredicateFields obj={artifact.predicate} />}
    </Stack>
  )
}

function useElapsed(since?: string): { formatted: string; seconds: number } | null {
  const [now, setNow] = useState(() => Date.now())

  useEffect(() => {
    if (!since) return
    const id = setInterval(() => setNow(Date.now()), 1000)
    return () => clearInterval(id)
  }, [since])

  if (!since) return null
  const seconds = Math.max(0, Math.floor((now - new Date(since).getTime()) / 1000))
  const m = Math.floor(seconds / 60)
  const s = seconds % 60
  return { seconds, formatted: m > 0 ? `${m}m ${s}s` : `${s}s` }
}

function MessageBubble({ job }: { job: Job }) {
  const isPending = job.status === 'pending'
  const isWorking = job.status === 'claimed' || job.status === 'running'
  const pendingElapsed = useElapsed(isPending ? job.createdAt : undefined)
  const workingElapsed = useElapsed(isWorking ? (job.claimedAt ?? job.createdAt) : undefined)

  return (
    <Stack gap="xs">
      <Paper withBorder radius="md" p="sm" ml="20%" bg="var(--mantine-color-blue-light)">
        <Text size="sm">{job.input}</Text>
      </Paper>
      <Paper withBorder radius="md" p="sm" mr="20%">
        <Group gap="xs" mb={4}>
          <Badge color={JOB_STATUS_COLOR[job.status] ?? 'gray'} size="sm">
            {job.status}
          </Badge>
          <Text size="xs" c="dimmed">
            {job.department}
          </Text>
        </Group>
        {isPending && (
          <Group gap="xs">
            <Loader size="xs" />
            <Text size="sm" c="dimmed">
              queued{pendingElapsed ? ` - ${pendingElapsed.formatted}` : ''}, waiting for a worker
            </Text>
          </Group>
        )}
        {isWorking && (
          <Stack gap={2}>
            <Group gap="xs">
              <Loader size="xs" />
              <Text size="sm" c="dimmed">
                working... {workingElapsed?.formatted}
              </Text>
            </Group>
            {workingElapsed && workingElapsed.seconds > 90 && (
              <Text size="xs" c="dimmed">
                real investigation can take several minutes - still going
              </Text>
            )}
          </Stack>
        )}
        {job.status === 'done' && job.resultDigest && <AssistantResult digest={job.resultDigest} />}
        {job.status === 'failed' && (
          <Text size="sm" c="red">
            {job.detail ?? 'failed - no detail recorded'}
          </Text>
        )}
        {job.status === 'escalated' && (
          <Text size="sm" c="orange">
            escalated - needs human review (see Exceptions)
          </Text>
        )}
        {job.status === 'quarantined' && (
          <Text size="sm" c="grape">
            quarantined - failed repeatedly with no fix in between, stopped retrying (see Exceptions)
          </Text>
        )}
      </Paper>
    </Stack>
  )
}

function ChatPanel({
  department,
  jobs,
  onSent,
  onUnauthorized,
}: {
  department: Department
  jobs: Job[]
  onSent: () => void
  onUnauthorized: () => void
}) {
  const [input, setInput] = useState('')
  const [sending, setSending] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [note, setNote] = useState<string | null>(null)

  const thread = useMemo(
    () => jobs.filter((j) => j.department === department.name).slice().reverse(),
    [jobs, department.name],
  )

  const send = async () => {
    if (!input.trim()) return
    setSending(true)
    setError(null)
    setNote(null)
    try {
      const res = await runDepartment(department.name, input)
      if (res.status === 'already_queued') {
        setNote('already queued - a matching job is already pending')
      } else if (res.status === 'escalated') {
        setNote(`escalated: ${res.reasons?.join('; ') ?? 'needs review'}`)
      } else if (res.status === 'refused') {
        setNote(`refused: ${res.reasons?.join('; ') ?? 'policy declined'}`)
      } else {
        setInput('')
      }
      onSent()
    } catch (e) {
      if (e instanceof UnauthorizedError) {
        onUnauthorized()
      } else {
        setError(e instanceof Error ? e.message : String(e))
      }
    } finally {
      setSending(false)
    }
  }

  return (
    <Stack h="100%" gap={0}>
      <Group px="md" py="xs" style={{ borderBottom: '1px solid var(--mantine-color-default-border)' }}>
        <Title order={4}>{department.name}</Title>
        <Badge color={CONSEQUENCE_COLOR[department.risk] ?? 'gray'}>{department.risk}</Badge>
        <Badge variant="outline">{department.reversibility}</Badge>
        <Text size="xs" c="dimmed">
          {department.procedure}
        </Text>
      </Group>

      <ScrollArea flex={1} p="md">
        {thread.length === 0 && (
          <Text c="dimmed" ta="center" mt="xl">
            No messages yet - send {department.name} something to do.
          </Text>
        )}
        <Stack gap="md">
          {thread.map((j) => (
            <MessageBubble key={j.id} job={j} />
          ))}
        </Stack>
      </ScrollArea>

      <Stack p="md" gap="xs" style={{ borderTop: '1px solid var(--mantine-color-default-border)' }}>
        {error && <Alert color="red">{error}</Alert>}
        {note && (
          <Text size="sm" c="dimmed">
            {note}
          </Text>
        )}
        <Group align="flex-end" gap="xs">
          <Textarea
            flex={1}
            placeholder={`Message ${department.name}...`}
            value={input}
            onChange={(e) => setInput(e.currentTarget.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter' && !e.shiftKey) {
                e.preventDefault()
                send()
              }
            }}
            autosize
            minRows={1}
            maxRows={6}
          />
          <ActionIcon size="lg" onClick={send} loading={sending} disabled={!input.trim()}>
            ➤
          </ActionIcon>
        </Group>
      </Stack>
    </Stack>
  )
}

function ExceptionDetail({ exc, onResolved }: { exc: Exception; onResolved: () => void }) {
  const [resolving, setResolving] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)

  const choose = async (option: string) => {
    if (!window.confirm(`Resolve ${exc.department} with: "${option}"?`)) return
    setResolving(option)
    setError(null)
    try {
      await resolveException(exc.digest, option)
      onResolved()
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
      setResolving(null)
    }
  }

  return (
    <Stack gap="xs" mt="sm">
      <Text size="sm">
        <b>because</b> {exc.because}
      </Text>
      <Text size="sm">
        <b>recommendation</b> {exc.recommendation}
      </Text>
      {exc.uncertainty && (
        <Text size="sm">
          <b>uncertainty</b> {exc.uncertainty}
        </Text>
      )}
      {exc.deadline && (
        <Text size="sm">
          <b>deadline</b> {exc.deadline}
        </Text>
      )}
      {exc.requiredAuthority && (
        <Text size="sm">
          <b>requires</b> {exc.requiredAuthority}
        </Text>
      )}
      {error && <Alert color="red">{error}</Alert>}
      <Group>
        {exc.options.map((o) => (
          <Button key={o} onClick={() => choose(o)} loading={resolving === o} disabled={resolving !== null}>
            {o}
          </Button>
        ))}
      </Group>
    </Stack>
  )
}

function ExceptionsPanel({ onUnauthorized }: { onUnauthorized: () => void }) {
  const [exceptions, setExceptions] = useState<Exception[] | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [expanded, setExpanded] = useState<string | null>(null)

  const reload = () => {
    setError(null)
    listExceptions()
      .then(setExceptions)
      .catch((e) => {
        if (e instanceof UnauthorizedError) return onUnauthorized()
        setError(e instanceof Error ? e.message : String(e))
      })
  }

  useEffect(reload, [])
  useEffect(() => subscribeToExceptionChanges(reload), [])

  return (
    <ScrollArea h="100%" p="md">
      <Title order={4} mb="md">
        Exceptions
      </Title>
      {error && (
        <Alert color="red" mb="md">
          {error}
        </Alert>
      )}
      {exceptions === null && !error && <Loader />}
      {exceptions !== null && exceptions.length === 0 && <Text c="dimmed">No open exceptions.</Text>}
      <Stack gap="sm">
        {exceptions?.map((exc) => (
          <Paper
            key={exc.digest}
            withBorder
            radius="md"
            p="md"
            onClick={() => setExpanded(expanded === exc.digest ? null : exc.digest)}
            style={{ cursor: 'pointer' }}
          >
            <Group justify="space-between">
              <Group>
                <Badge color={CONSEQUENCE_COLOR[exc.consequence] ?? 'gray'}>{exc.consequence}</Badge>
                <Text size="sm" c="dimmed">
                  {exc.class}
                </Text>
                <Text fw={600}>{exc.department}</Text>
              </Group>
              {exc.deadline && (
                <Text size="sm" c="dimmed">
                  {exc.deadline}
                </Text>
              )}
            </Group>
            {expanded !== exc.digest && (
              <Text size="sm" mt={4} lineClamp={1}>
                {exc.because}
              </Text>
            )}
            {expanded === exc.digest && (
              <div onClick={(e) => e.stopPropagation()}>
                <ExceptionDetail
                  exc={exc}
                  onResolved={() => {
                    setExpanded(null)
                    reload()
                  }}
                />
              </div>
            )}
          </Paper>
        ))}
      </Stack>
    </ScrollArea>
  )
}

function HistoryItem({ artifact }: { artifact: Artifact }) {
  const [expanded, setExpanded] = useState(false)
  const preview =
    typeof artifact.predicate.claim === 'string'
      ? artifact.predicate.claim
      : typeof artifact.predicate.outcome === 'string'
        ? artifact.predicate.outcome
        : typeof artifact.predicate.intent === 'string'
          ? artifact.predicate.intent
          : null

  return (
    <Paper withBorder radius="md" p="md" onClick={() => setExpanded((v) => !v)} style={{ cursor: 'pointer' }}>
      <Group justify="space-between" wrap="nowrap">
        <Group gap="xs" wrap="nowrap">
          <Badge variant="light">{artifact.kind}</Badge>
          {preview && !expanded && (
            <Text size="sm" c="dimmed" lineClamp={1}>
              {preview}
            </Text>
          )}
        </Group>
        <Text size="xs" c="dimmed">
          {artifact.id}
        </Text>
      </Group>
      {expanded && (
        <div onClick={(e) => e.stopPropagation()}>
          <Box mt="sm">
            <PredicateFields obj={artifact.predicate} />
          </Box>
        </div>
      )}
    </Paper>
  )
}

function HistoryPanel({ onUnauthorized }: { onUnauthorized: () => void }) {
  const [artifacts, setArtifacts] = useState<Artifact[]>([])
  const [nextBefore, setNextBefore] = useState<string | undefined>(undefined)
  const [kind, setKind] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const load = (before: string | undefined, replace: boolean) => {
    setLoading(true)
    setError(null)
    listArtifactHistory({ before, kind: kind ?? undefined })
      .then((page) => {
        setArtifacts((prev) => (replace ? page.artifacts : [...prev, ...page.artifacts]))
        setNextBefore(page.nextBefore)
      })
      .catch((e) => {
        if (e instanceof UnauthorizedError) return onUnauthorized()
        setError(e instanceof Error ? e.message : String(e))
      })
      .finally(() => setLoading(false))
  }

  useEffect(() => {
    load(undefined, true)
  }, [kind])

  return (
    <ScrollArea h="100%" p="md">
      <Group justify="space-between" mb="md">
        <Title order={4}>History</Title>
        <Select
          placeholder="All kinds"
          data={HISTORY_KINDS.map((k) => ({ value: k, label: k }))}
          value={kind}
          onChange={setKind}
          clearable
          w={180}
        />
      </Group>
      {error && (
        <Alert color="red" mb="md">
          {error}
        </Alert>
      )}
      {artifacts.length === 0 && !loading && <Text c="dimmed">No artifacts yet.</Text>}
      <Stack gap="sm">
        {artifacts.map((a) => (
          <HistoryItem key={a.id} artifact={a} />
        ))}
      </Stack>
      {loading && (
        <Group justify="center" mt="md">
          <Loader size="sm" />
        </Group>
      )}
      {!loading && nextBefore && (
        <Group justify="center" mt="md">
          <Button variant="subtle" onClick={() => load(nextBefore, false)}>
            Load more
          </Button>
        </Group>
      )}
    </ScrollArea>
  )
}

export default function App() {
  const [departments, setDepartments] = useState<Department[] | null>(null)
  const [jobs, setJobs] = useState<Job[]>([])
  const [selected, setSelected] = useState<string | null>(null)
  const [view, setView] = useState<View>(VIEW_DEPARTMENT)
  const [exceptionCount, setExceptionCount] = useState(0)
  const [error, setError] = useState<string | null>(null)
  const [health, setHealth] = useState<Health | null>(null)
  const [connected, setConnected] = useState(true)
  const [authed, setAuthed] = useState(() => !!getToken())

  const handleUnauthorized = () => {
    clearToken()
    setAuthed(false)
  }

  useEffect(() => {
    let cancelled = false
    const poll = () => {
      getHealth()
        .then((h) => {
          if (cancelled) return
          setHealth(h)
          setConnected(true)
        })
        .catch(() => {
          if (cancelled) return
          setConnected(false)
        })
    }
    poll()
    const id = setInterval(poll, 5000)
    return () => {
      cancelled = true
      clearInterval(id)
    }
  }, [])

  useEffect(() => {
    if (!authed) return
    listDepartments()
      .then((d) => {
        setDepartments(d)
        if (d.length > 0) setSelected(d[0].name)
      })
      .catch((e) => {
        if (e instanceof UnauthorizedError) return handleUnauthorized()
        setError(e instanceof Error ? e.message : String(e))
      })
  }, [authed])

  const reloadJobs = () => {
    listJobs()
      .then(setJobs)
      .catch((e) => {
        if (e instanceof UnauthorizedError) return handleUnauthorized()
        setError(e instanceof Error ? e.message : String(e))
      })
  }

  useEffect(() => {
    if (authed) reloadJobs()
  }, [authed])
  useEffect(() => {
    if (!authed) return
    return subscribeToJobChanges(reloadJobs)
  }, [authed])

  useEffect(() => {
    if (!authed) return
    listExceptions()
      .then((e) => setExceptionCount(e.length))
      .catch((e) => {
        if (e instanceof UnauthorizedError) handleUnauthorized()
      })
    return subscribeToExceptionChanges(() => {
      listExceptions()
        .then((e) => setExceptionCount(e.length))
        .catch(() => {})
    })
  }, [authed])

  const activeDepartment = departments?.find((d) => d.name === selected) ?? null

  if (!authed) {
    return <TokenGate onSaved={() => setAuthed(true)} />
  }

  return (
    <AppShell header={{ height: 52 }} navbar={{ width: 240, breakpoint: 0 }} padding={0}>
      <AppShell.Header>
        <Group h="100%" px="md" justify="space-between">
          <Title order={4}>hr</Title>
          <Group gap="md">
            <StatusPill health={health} connected={connected} />
            <Button variant="subtle" size="compact-xs" onClick={handleUnauthorized}>
              Sign out
            </Button>
          </Group>
        </Group>
      </AppShell.Header>

      <AppShell.Navbar p="xs">
        <Text size="xs" c="dimmed" tt="uppercase" fw={700} px="xs" mb={4}>
          Departments
        </Text>
        <Stack gap={2}>
          {departments?.map((d) => (
            <NavLink
              key={d.name}
              label={d.name}
              description={d.risk}
              active={view === VIEW_DEPARTMENT && selected === d.name}
              onClick={() => {
                setSelected(d.name)
                setView(VIEW_DEPARTMENT)
              }}
            />
          ))}
        </Stack>
        <Divider my="sm" />
        <NavLink
          label="Exceptions"
          active={view === VIEW_EXCEPTIONS}
          onClick={() => setView(VIEW_EXCEPTIONS)}
          rightSection={exceptionCount > 0 ? <Badge color="orange">{exceptionCount}</Badge> : undefined}
        />
        <NavLink label="History" active={view === VIEW_HISTORY} onClick={() => setView(VIEW_HISTORY)} />
      </AppShell.Navbar>

      <AppShell.Main h="100vh">
        {!connected && <DisconnectedPanel />}
        {connected && error && (
          <Alert color="red" m="md">
            {error}
          </Alert>
        )}
        {connected && view === VIEW_EXCEPTIONS && <ExceptionsPanel onUnauthorized={handleUnauthorized} />}
        {connected && view === VIEW_HISTORY && <HistoryPanel onUnauthorized={handleUnauthorized} />}
        {connected && view === VIEW_DEPARTMENT && activeDepartment && (
          <ChatPanel department={activeDepartment} jobs={jobs} onSent={reloadJobs} onUnauthorized={handleUnauthorized} />
        )}
        {connected && view === VIEW_DEPARTMENT && !activeDepartment && departments === null && (
          <Stack align="center" mt="xl">
            <Loader />
          </Stack>
        )}
      </AppShell.Main>
    </AppShell>
  )
}
