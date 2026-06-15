interface HeaderProps {
  onMenuClick: () => void;
}

export default function Header({ onMenuClick }: HeaderProps) {
  return (
    <header className="flex h-16 items-center gap-4 border-b border-gray-200 bg-white px-4 dark:border-gray-700 dark:bg-gray-800">
      <button
        onClick={onMenuClick}
        className="rounded-lg p-2 text-gray-500 hover:bg-gray-100 dark:hover:bg-gray-700 lg:hidden"
      >
        ☰
      </button>
      <h2 className="text-lg font-semibold text-gray-800 dark:text-white">
        Download Manager
      </h2>
      {/* You can add global actions here later */}
    </header>
  );
}