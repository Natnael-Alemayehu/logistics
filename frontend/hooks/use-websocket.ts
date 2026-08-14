import { useEffect, useRef, useCallback, useState, useMemo } from 'react'
import { useAuthStore } from '@/stores/auth-store'

export type ConnectionStatus = 'connecting' | 'connected' | 'disconnected' | 'error'

export interface WebSocketMessage<T = unknown> {
  type: string
  data?: T
  channel?: string
  action?: string
}

export interface DriverLocationUpdate {
  driver_id: string
  lat: number
  lng: number
  speed?: number
  timestamp: string
}

export interface SubscribeMessage {
  action: 'subscribe' | 'unsubscribe'
  channel: string
}

interface QueuedMessage {
  data: string
  timestamp: number
}

interface WebSocketOptions {
  onOpen?: () => void
  onClose?: (event: CloseEvent) => void
  onError?: (error: Event) => void
  onReconnecting?: (attempt: number) => void
  reconnect?: boolean
  initialReconnectDelay?: number
  maxReconnectDelay?: number
  maxReconnectAttempts?: number
  heartbeatInterval?: number
  enableMessageQueue?: boolean
  debug?: boolean
}

const DEFAULT_INITIAL_RECONNECT_DELAY = 1000
const DEFAULT_MAX_RECONNECT_DELAY = 30000
const DEFAULT_HEARTBEAT_INTERVAL = 30000
const DEFAULT_MAX_RECONNECT_ATTEMPTS = Infinity

export function useWebSocket(options: WebSocketOptions = {}) {
  const {
    onOpen,
    onClose,
    onError,
    onReconnecting,
    reconnect = true,
    initialReconnectDelay = DEFAULT_INITIAL_RECONNECT_DELAY,
    maxReconnectDelay = DEFAULT_MAX_RECONNECT_DELAY,
    maxReconnectAttempts = DEFAULT_MAX_RECONNECT_ATTEMPTS,
    heartbeatInterval = DEFAULT_HEARTBEAT_INTERVAL,
    enableMessageQueue = true,
    debug = false,
  } = options

  const socketRef = useRef<WebSocket | null>(null)
  const auth = useAuthStore()
  const accessToken = auth.accessToken
  const reconnectTimeoutRef = useRef<NodeJS.Timeout | null>(null)
  const reconnectAttemptsRef = useRef(0)
  const heartbeatTimeoutRef = useRef<NodeJS.Timeout | null>(null)
  const messageQueueRef = useRef<QueuedMessage[]>([])
  const isManualCloseRef = useRef(false)
  const subscribersRef = useRef<Map<string, Set<(data: unknown) => void>>>(new Map())
  
  const [status, setStatus] = useState<ConnectionStatus>('disconnected')

  const log = useCallback((...args: unknown[]) => {
    if (debug) {
      console.log('[WebSocket]', ...args)
    }
  }, [debug])

  const clearHeartbeat = useCallback(() => {
    if (heartbeatTimeoutRef.current) {
      clearTimeout(heartbeatTimeoutRef.current)
      heartbeatTimeoutRef.current = null
    }
  }, [])

  const startHeartbeat = useCallback(() => {
    clearHeartbeat()
    
    heartbeatTimeoutRef.current = setTimeout(() => {
      if (socketRef.current?.readyState === WebSocket.OPEN) {
        log('Sending heartbeat ping')
        socketRef.current.send(JSON.stringify({ type: 'ping' }))
        startHeartbeat()
      }
    }, heartbeatInterval)
  }, [clearHeartbeat, heartbeatInterval, log])

  const flushMessageQueue = useCallback(() => {
    if (!enableMessageQueue || messageQueueRef.current.length === 0) return

    const queue = [...messageQueueRef.current]
    messageQueueRef.current = []

    queue.forEach(({ data }) => {
      if (socketRef.current?.readyState === WebSocket.OPEN) {
        log('Flushing queued message')
        socketRef.current.send(data)
      }
    })
  }, [enableMessageQueue, log])

  const getReconnectDelay = useCallback(() => {
    const delay = Math.min(
      initialReconnectDelay * Math.pow(2, reconnectAttemptsRef.current),
      maxReconnectDelay
    )
    return delay + Math.random() * 1000
  }, [initialReconnectDelay, maxReconnectDelay])

  const connect = useCallback(() => {
    if (!accessToken) {
      log('No access token, skipping connection')
      return
    }

    if (socketRef.current?.readyState === WebSocket.OPEN) {
      log('Already connected')
      return
    }

    isManualCloseRef.current = false
    const wsUrl = getWebSocketUrl(accessToken)
    
    log('Connecting to', wsUrl.replace(/token=[^&]+/, 'token=***'))
    setStatus('connecting')

    try {
      const socket = new WebSocket(wsUrl)
      socketRef.current = socket

      socket.onopen = () => {
        log('Connected')
        setStatus('connected')
        reconnectAttemptsRef.current = 0
        
        subscribersRef.current.forEach((_, channel) => {
          socket.send(JSON.stringify({ action: 'subscribe', channel }))
        })
        
        flushMessageQueue()
        startHeartbeat()
        onOpen?.()
      }

      socket.onmessage = (event) => {
        try {
          const message: WebSocketMessage = JSON.parse(event.data)
          
          if (message.type === 'pong') {
            log('Received pong')
            return
          }

          if (message.channel && subscribersRef.current.has(message.channel)) {
            const handlers = subscribersRef.current.get(message.channel)
            handlers?.forEach((handler) => handler(message.data ?? message))
          } else {
            const allHandlers = subscribersRef.current.get('*')
            allHandlers?.forEach((handler) => handler(message))
          }
        } catch (e) {
          log('Failed to parse message', e)
        }
      }

      socket.onclose = (event) => {
        log('Disconnected', event.code, event.reason)
        clearHeartbeat()
        setStatus('disconnected')
        onClose?.(event)

        if (!isManualCloseRef.current && reconnect && reconnectAttemptsRef.current < maxReconnectAttempts) {
          const delay = getReconnectDelay()
          reconnectAttemptsRef.current++
          log(`Reconnecting in ${delay}ms (attempt ${reconnectAttemptsRef.current})`)
          onReconnecting?.(reconnectAttemptsRef.current)
          
          reconnectTimeoutRef.current = setTimeout(connect, delay)
        }
      }

      socket.onerror = (error) => {
        log('Error', error)
        setStatus('error')
        onError?.(error)
      }
    } catch (error) {
      log('Failed to create WebSocket', error)
      setStatus('error')
    }
  }, [
    accessToken,
    log,
    flushMessageQueue,
    startHeartbeat,
    clearHeartbeat,
    onClose,
    onOpen,
    onError,
    onReconnecting,
    reconnect,
    maxReconnectAttempts,
    getReconnectDelay,
  ])

  const disconnect = useCallback(() => {
    isManualCloseRef.current = true
    
    if (reconnectTimeoutRef.current) {
      clearTimeout(reconnectTimeoutRef.current)
      reconnectTimeoutRef.current = null
    }
    
    clearHeartbeat()
    
    if (socketRef.current) {
      socketRef.current.close()
      socketRef.current = null
    }
    
    setStatus('disconnected')
    reconnectAttemptsRef.current = 0
  }, [clearHeartbeat])

  const send = useCallback(<T,>(data: T | WebSocketMessage<T>): boolean => {
    const messageStr = JSON.stringify(data)
    
    if (socketRef.current?.readyState === WebSocket.OPEN) {
      socketRef.current.send(messageStr)
      return true
    }
    
    if (enableMessageQueue) {
      log('Queuing message (not connected)')
      messageQueueRef.current.push({
        data: messageStr,
        timestamp: Date.now(),
      })
    }
    
    return false
  }, [enableMessageQueue, log])

  const subscribe = useCallback((channel: string, handler: (data: unknown) => void) => {
    if (!subscribersRef.current.has(channel)) {
      subscribersRef.current.set(channel, new Set())
      
      if (socketRef.current?.readyState === WebSocket.OPEN) {
        socketRef.current.send(JSON.stringify({ action: 'subscribe', channel }))
      }
    }
    
    subscribersRef.current.get(channel)?.add(handler)
    
    return () => {
      subscribersRef.current.get(channel)?.delete(handler)
      
      if (subscribersRef.current.get(channel)?.size === 0) {
        subscribersRef.current.delete(channel)
        
        if (socketRef.current?.readyState === WebSocket.OPEN) {
          socketRef.current.send(JSON.stringify({ action: 'unsubscribe', channel }))
        }
      }
    }
  }, [])

  useEffect(() => {
    if (accessToken) {
      connect()
    }
    
    return () => {
      disconnect()
    }
  }, [connect, disconnect, accessToken])

  return useMemo(() => ({
    status,
    send,
    disconnect,
    reconnect: connect,
    subscribe,
    isConnected: status === 'connected',
    isConnecting: status === 'connecting',
    isDisconnected: status === 'disconnected',
    hasError: status === 'error',
  }), [status, send, disconnect, connect, subscribe])
}

function getWebSocketUrl(token: string): string {
  const wsUrl = process.env.NEXT_PUBLIC_WS_URL || 'ws://localhost:8080'
  
  if (wsUrl.startsWith('ws://') || wsUrl.startsWith('wss://')) {
    const separator = wsUrl.includes('?') ? '&' : '?'
    return `${wsUrl}/api/v1/ws${separator}token=${token}`
  }
  
  const protocol = typeof window !== 'undefined' && window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const separator = wsUrl.includes('?') ? '&' : '?'
  return `${protocol}//${wsUrl}/api/v1/ws${separator}token=${token}`
}
