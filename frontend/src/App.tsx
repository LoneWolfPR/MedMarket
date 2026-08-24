import { Routes, Route } from 'react-router'
import Layout from './components/Layout'
import Home from './pages/Home'
import Login from './pages/Login'
import Register from './pages/Register'
import RequireAuth from './auth/RequireAuth'
import Prescriptions from './pages/Prescriptions'
import NotFound from './pages/NotFound'
import PriceSearch from './pages/PriceSearch'

function App() {
  return (
    <Routes>
      <Route path="/" element={<Layout />}>
        <Route index element={<Home />} />
        <Route path="login" element={<Login />} />
        <Route path="register" element={<Register />} />
        <Route element={<RequireAuth />}>
          <Route path="prescriptions" element={<Prescriptions />} />
          <Route path="prescriptions/:id/search" element={<PriceSearch />} />
        </Route>
        <Route path="*" element={<NotFound />} />
      </Route>
    </Routes>
  )
}

export default App
