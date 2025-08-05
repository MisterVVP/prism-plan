import { createContext, useContext, useEffect, useState } from 'react';

interface LayoutContextType {
  isMobile: boolean;
  isLarge: boolean;
}

const LayoutContext = createContext<LayoutContextType>({ isMobile: false, isLarge: true });

export function LayoutProvider({ children }: { children: React.ReactNode }) {
  const [width, setWidth] = useState<number>(typeof window !== 'undefined' ? window.innerWidth : 1024);

  useEffect(() => {
    function handleResize() {
      setWidth(window.innerWidth);
    }
    handleResize();
    window.addEventListener('resize', handleResize);
    return () => window.removeEventListener('resize', handleResize);
  }, []);

  const value = {
    isMobile: width < 640,
    isLarge: width >= 1024
  };

  return <LayoutContext.Provider value={value}>{children}</LayoutContext.Provider>;
}

export function useLayout() {
  return useContext(LayoutContext);
}
