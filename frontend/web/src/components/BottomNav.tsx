import { Link } from 'react-router-dom';
import { MessageSquare, Clock, Map, User, Users, Home } from 'lucide-react';
import { cn } from '@/lib/utils';
import { useRole } from '@/hooks/useRole';

interface BottomNavProps {
  active: 'dashboard' | 'chat' | 'history' | 'route' | 'profile' | 'colaborador';
}

export default function BottomNav({ active }: BottomNavProps) {
  // SUP-256: Misiones solo colaborador o admin.
  const { canAccessColaborador } = useRole();
  const navItems = [
    { id: 'dashboard', label: 'Inicio', icon: Home, to: '/dashboard' },
    { id: 'chat', label: 'Chat IA', icon: MessageSquare, to: '/chat' },
    { id: 'history', label: 'Listas', icon: Clock, to: '/history' },
    { id: 'route', label: 'Ruta', icon: Map, to: '/route' },
    ...(canAccessColaborador ? [{ id: 'colaborador', label: 'Misiones', icon: Users, to: '/colaborador' }] : []),
    { id: 'profile', label: 'Perfil', icon: User, to: '/profile' }
  ];

  return (
    <div className="fixed bottom-0 left-0 right-0 bg-background border-t border-border pb-safe z-50">
      <nav className="flex items-center justify-around px-2 h-16 max-w-lg mx-auto">
        {navItems.map(item => {
          const isActive = active === item.id;
          return (
            <Link 
              key={item.id} 
              to={item.to}
              className={cn(
                "flex flex-col items-center justify-center w-full h-full space-y-1 transition-colors",
                isActive ? "text-primary" : "text-muted-foreground hover:text-foreground"
              )}
            >
              <item.icon className={cn("h-5 w-5", isActive && "fill-current/20 stroke-[2.5px]")} />
              <span className={cn("text-[10px]", isActive ? "font-bold" : "font-medium")}>
                {item.label}
              </span>
            </Link>
          );
        })}
      </nav>
    </div>
  );
}
