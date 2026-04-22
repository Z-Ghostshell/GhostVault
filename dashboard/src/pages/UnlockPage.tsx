import { useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { useSessionUI } from '../context/SessionUIContext'

/** `/unlock` opens the session modal and replaces the URL with `/` for bookmarks. */
export default function UnlockPage() {
  const navigate = useNavigate()
  const { openSessionModal } = useSessionUI()

  useEffect(() => {
    openSessionModal()
    navigate('/', { replace: true })
  }, [navigate, openSessionModal])

  return null
}
