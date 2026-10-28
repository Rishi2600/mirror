import { createBrowserRouter } from 'react-router'
import { RequireUser } from './components/RequireUser'
import { ChooseRolePage } from './pages/ChooseRolePage'
import { HiringCardPage } from './pages/HiringCardPage'
import { HiringTimelinePage } from './pages/HiringTimelinePage'
import { HomePage } from './pages/HomePage'
import { NewHiringCardPage } from './pages/NewHiringCardPage'
import { NotFoundPage } from './pages/NotFoundPage'
import { ProfilePage } from './pages/ProfilePage'
import { SignInPage } from './pages/SignInPage'
import { TimelinePage } from './pages/TimelinePage'

export const router = createBrowserRouter([
  { path: '/sign-in', element: <SignInPage /> },
  {
    element: <RequireUser needsRole={false} />,
    children: [{ path: '/choose-role', element: <ChooseRolePage /> }],
  },
  {
    element: <RequireUser needsRole />,
    children: [
      { path: '/', element: <HomePage /> },
      { path: '/profile', element: <ProfilePage /> },
      { path: '/hiring/new', element: <NewHiringCardPage /> },
      { path: '/hiring/:id', element: <HiringCardPage /> },
      { path: '/hiring/:id/timeline', element: <HiringTimelinePage /> },
      { path: '/timeline', element: <TimelinePage /> },
    ],
  },
  { path: '*', element: <NotFoundPage /> },
])
