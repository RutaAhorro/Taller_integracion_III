import { useState, type FormEvent, useEffect } from 'react'
import { Menu, MapPin, Search, ShoppingBasket, CircleUserRound, Bot, User, LogOut } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Link, useNavigate } from 'react-router-dom'
import { ROUTES } from '@/core/routes'
import { useAuth } from '@/hooks/useAuth'

type TopNavProps = {
    query: string
    onQueryChange: (value: string) => void
    onMenuClick: () => void
    onResetFilters: () => void
}

export function TopNav({ query, onQueryChange, onMenuClick, onResetFilters }: TopNavProps) {
    const [isDropdownOpen, setIsDropdownOpen] = useState(false);
    const [localQuery, setLocalQuery] = useState(query || '');
    const navigate = useNavigate();
    const { logout } = useAuth();
    
    // Fallbacks para props futuras (previene hardcodeos strictos)
    const activeCity = "Temuco";

    useEffect(() => {
        setLocalQuery(query || '');
    }, [query]);

    const handleSearch = (e: FormEvent) => {
        e.preventDefault();
        onQueryChange(localQuery);
        navigate(ROUTES.CATALOGO);
    };

    return (
        <header className="sticky top-0 z-50 border-b border-border bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/80">
            <div className="mx-auto flex h-16 max-w-[1600px] items-center gap-3 px-4 md:gap-6 md:px-6">
                <Button
                    variant="ghost"
                    size="icon"
                    className="hidden md:flex h-10 w-10 shrink-0 rounded-full"
                    onClick={onMenuClick}
                    aria-label="Abrir menú principal"
                >
                    <Menu className="h-5 w-5" aria-hidden="true" />
                </Button>

                <Link
                    to="/dashboard"
                    aria-label="Ir al inicio"
                    className="flex items-center gap-2 shrink-0"
                    onClick={onResetFilters}
                >
                    <span className="flex h-9 w-9 items-center justify-center rounded-lg bg-primary text-primary-foreground shadow-sm">
                        <ShoppingBasket className="h-5 w-5" aria-hidden="true" />
                    </span>
                    <span className="hidden text-lg font-semibold tracking-tight sm:inline text-foreground">
                        Ruta<span className="text-primary">Ahorro</span>
                    </span>
                </Link>

                <form
                    role="search"
                    className="relative flex-1"
                    onSubmit={handleSearch}
                >
                    <label htmlFor="product-search" className="sr-only">
                        Buscar productos
                    </label>
                    <Search
                        className="pointer-events-none absolute left-4 top-1/2 h-5 w-5 -translate-y-1/2 text-muted-foreground"
                        aria-hidden="true"
                    />
                    <input
                        id="product-search"
                        type="search"
                        value={localQuery}
                        onChange={(e) => setLocalQuery(e.target.value)}
                        placeholder="Buscar productos"
                        autoComplete="off"
                        className="h-11 w-full rounded-full border border-input bg-background pl-10 md:pl-12 pr-12 md:pr-28 text-sm md:text-base outline-none transition-shadow placeholder:text-muted-foreground focus-visible:ring-2 focus-visible:ring-ring"
                    />
                    <Button
                        type="submit"
                        size="sm"
                        className="absolute right-1.5 top-1/2 h-8 w-8 md:w-auto -translate-y-1/2 hover:-translate-y-1/2 hover:scale-105 hover:brightness-110 active:-translate-y-1/2 active:scale-95 hover:shadow-none active:shadow-none rounded-full p-0 md:px-4 flex items-center justify-center transition-all"
                        aria-label="Ejecutar búsqueda"
                    >
                        <span className="hidden md:inline">Buscar</span>
                        <Search className="h-4 w-4 md:hidden" />
                    </Button>
                </form>

                <Link
                    to="/chat"
                    className="hidden sm:flex items-center gap-2 h-11 px-4 rounded-full bg-primary hover:brightness-110 hover:scale-105 text-primary-foreground font-semibold transition-all shrink-0 shadow-sm focus-visible:ring-2 focus-visible:ring-offset-2"
                >
                    <Bot className="h-5 w-5" />
                    <span>Asistente IA</span>
                </Link>

                <div className="flex items-center gap-2 shrink-0">
                    <Button
                        variant="outline"
                        className="hidden h-12 gap-2 rounded-full px-4 md:flex pointer-events-none"
                    >
                        <MapPin className="h-4 w-4 shrink-0 text-muted-foreground" aria-hidden="true" />
                        <div className="flex flex-col text-left">
                            <span className="text-[10px] font-medium leading-none text-muted-foreground">
                                Comparando en
                            </span>
                            <span className="text-xs font-semibold leading-tight text-foreground">
                                {activeCity}
                            </span>
                        </div>
                    </Button>
                    <div className="relative">
                        <Button
                            variant="ghost"
                            size="icon"
                            className="h-10 w-10 rounded-full border border-border shadow-sm bg-background hover:bg-accent transition-colors"
                            aria-label="Menú de usuario"
                            onClick={() => setIsDropdownOpen(!isDropdownOpen)}
                        >
                            <CircleUserRound className="h-6 w-6 text-foreground/80" aria-hidden="true" />
                        </Button>

                        {isDropdownOpen && (
                            <>
                                <div className="fixed inset-0 z-40" onClick={() => setIsDropdownOpen(false)} />
                                <div className="absolute right-0 mt-2 w-48 bg-card border border-border rounded-xl shadow-lg z-50 overflow-hidden animate-in fade-in zoom-in-95 duration-200">
                                    <Link
                                        to="/profile"
                                        onClick={() => setIsDropdownOpen(false)}
                                        className="flex items-center gap-3 px-4 py-3 text-sm font-medium hover:bg-accent transition-colors"
                                    >
                                        <User className="h-4 w-4" />
                                        Mi Perfil
                                    </Link>
                                    <div className="h-px bg-border w-full" />
                                    <Link to={ROUTES.LISTAS} onClick={() => setIsDropdownOpen(false)} className="flex items-center gap-3 px-4 py-3 text-sm font-medium hover:bg-accent transition-colors">
                                        <ShoppingBasket className="h-4 w-4" />
                                        Mis listas
                                    </Link>
                                    <button
                                        onClick={() => {
                                            setIsDropdownOpen(false);
                                            logout();
                                            navigate('/login');
                                        }}
                                        className="flex w-full items-center gap-3 px-4 py-3 text-sm font-medium text-destructive hover:bg-destructive/10 transition-colors"
                                    >
                                        <LogOut className="h-4 w-4" />
                                        Cerrar Sesión
                                    </button>
                                </div>
                            </>
                        )}
                    </div>
                </div>
            </div>
        </header>
    )
}
