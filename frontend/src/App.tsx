import { Routes, Route } from 'react-router'
import Layout from './components/Layout'
import Home from './pages/Home'
import Login from './pages/Login'
import RequireAuth from './auth/RequireAuth'
import Prescriptions from './pages/Prescriptions'
function App() {
  return (
    <Routes>
      <Route path="/" element={<Layout />}>
        <Route index element={<Home />} />
        <Route path="login" element={<Login />} />
        <Route element={<RequireAuth />}>
          <Route path="prescriptions" element={<Prescriptions />} />
        </Route>
      </Route>
    </Routes>
  )
}

export default App
