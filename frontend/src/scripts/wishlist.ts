  const games = Array.from(document.querySelectorAll(".game-card") as NodeListOf<HTMLElement>);
  const searchInput = document.getElementById("search") as HTMLInputElement | null;
  const gameCountElement = document.getElementById("count");
  const noResultsElement = document.getElementById("no-results");
  const noResultsMessageElement = document.getElementById("no-results-message");
  const newFilterButton = document.getElementById("new-filter");
  const ownedButton = document.getElementById("owned");
  const searchClearButton = document.getElementById("search-clear");
  let updateQueryParamsTimeout: number | null = null;

  function updateSearchClearButton() {
    if (searchInput && searchClearButton) {
      if (searchInput.value.trim()) {
        searchClearButton.classList.remove('hidden');
      } else {
        searchClearButton.classList.add('hidden');
      }
    }
  }

  if (searchInput) {
    searchInput.addEventListener('input', updateSearchClearButton);
  }

  if (searchClearButton) {
    searchClearButton.addEventListener('click', () => {
      if (searchInput) {
        searchInput.value = '';
        updateSearchClearButton();
        filterGames();
        updateQueryParams();
      }
    });
  }

  updateSearchClearButton();

  let advancedFilters = {
    showOnlyNew: false,
    showOnlyOwned: false,
    playerCount: null as number | null,
    priorities: [] as number[],
    priceRanges: [] as string[],
    timeRanges: [] as string[],
    categories: [] as string[],
    mechanics: [] as string[]
  };

  function updateQueryParams() {
    if (updateQueryParamsTimeout !== null) {
      clearTimeout(updateQueryParamsTimeout);
    }

    updateQueryParamsTimeout = window.setTimeout(() => {
      const params = new URLSearchParams(window.location.search);
      const searchTerm = searchInput?.value?.trim() || "";

      if (searchTerm) {
        params.set("q", searchTerm);
      } else {
        params.delete("q");
      }

      if (advancedFilters.showOnlyNew) {
        params.set("new", "true");
      } else {
        params.delete("new");
      }

      if (advancedFilters.showOnlyOwned) {
        params.set("owned", "true");
      } else {
        params.delete("owned");
      }

      if (advancedFilters.playerCount !== null) {
        params.set("players", advancedFilters.playerCount.toString());
      } else {
        params.delete("players");
      }

      if (advancedFilters.priorities.length > 0) {
        params.set("priority", advancedFilters.priorities.join(','));
      } else {
        params.delete("priority");
      }

      if (advancedFilters.priceRanges.length > 0) {
        params.set("price", advancedFilters.priceRanges.join(','));
      } else {
        params.delete("price");
      }

      if (advancedFilters.timeRanges.length > 0) {
        params.set("time", advancedFilters.timeRanges.join(','));
      } else {
        params.delete("time");
      }

      const newUrl = `${window.location.pathname}${params.toString() ? '?' + params.toString() : ''}`;
      if (newUrl !== window.location.pathname + window.location.search) {
        window.history.pushState({}, '', newUrl);
      }

      updateQueryParamsTimeout = null;
    }, 500); // Delay to avoid too many updates
  }

  function applyQueryParams() {
    const params = new URLSearchParams(window.location.search);

    if (searchInput) {
      searchInput.value = params.get("q") || "";
    }

    const onlyNew = params.get("new");
    advancedFilters.showOnlyNew = onlyNew === "true";
    if (newFilterButton) {
      newFilterButton.dataset.active = onlyNew ? "true" : "false";
      newFilterButton.classList.toggle("invert", onlyNew === "true");
    }

    const onlyOwned = params.get("owned");
    advancedFilters.showOnlyOwned = onlyOwned === "true";
    if (ownedButton) {
      ownedButton.dataset.active = onlyOwned ? "true" : "false";
      ownedButton.classList.toggle("dark:bg-green-300", onlyOwned === "true");
      ownedButton.classList.toggle("dark:border-green-700", onlyOwned === "true");
      ownedButton.classList.toggle("dark:text-green-700", onlyOwned === "true");
      ownedButton.classList.toggle("dark:text-white", !onlyOwned);
    }

    const playersParam = params.get("players");
    if (playersParam) {
      advancedFilters.playerCount = parseInt(playersParam);
    }

    const priorityParam = params.get("priority");
    if (priorityParam) {
      advancedFilters.priorities = priorityParam.split(',').map(p => parseInt(p));
    }

    const priceParam = params.get("price");
    if (priceParam) {
      advancedFilters.priceRanges = priceParam.split(',');
    }

    const timeParam = params.get("time");
    if (timeParam) {
      advancedFilters.timeRanges = timeParam.split(',');
    }

    // Sync with modal
    if ((window as any).setFilterState) {
      (window as any).setFilterState(advancedFilters);
    }
  }

  function matchesPlayerCountRange(playersStr: string, targetCount: number): boolean {
    // Parse "2-4", "1-6", "4", etc.
    const match = playersStr.match(/(\d+)(?:-(\d+))?/);
    if (!match) return false;

    const min = parseInt(match[1]);
    const max = match[2] ? parseInt(match[2]) : min;

    return targetCount >= min && targetCount <= max;
  }

  function matchesPriceRange(price: number, ranges: string[]): boolean {
    return ranges.some(range => {
      const [min, max] = range.split('-').map(Number);
      return price >= min && price <= max;
    });
  }

  function matchesTimeRange(time: number, ranges: string[]): boolean {
    return ranges.some(range => {
      const [min, max] = range.split('-').map(Number);
      return time >= min && time <= max;
    });
  }

  function updateFilterBadges() {
    const badgesContainer = document.getElementById('filter-badges');
    if (!badgesContainer) return;

    badgesContainer.innerHTML = '';

    const badges: string[] = [];

    if (advancedFilters.showOnlyNew) {
      badges.push(`<button class="filter-badge px-3 py-1 rounded-full bg-indigo-100 dark:bg-indigo-900 text-indigo-800 dark:text-indigo-200 text-sm flex items-center gap-2 cursor-pointer" data-filter-type="showOnlyNew">
        <span>New games</span>
        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" class="size-4 remove-filter" data-filter-type="showOnlyNew">
          <path stroke-linecap="round" stroke-linejoin="round" d="M6 18 18 6M6 6l12 12" />
        </svg>
      </button>`);
    }

    if (advancedFilters.showOnlyOwned) {
      badges.push(`<button class="filter-badge px-3 py-1 rounded-full bg-indigo-100 dark:bg-indigo-900 text-indigo-800 dark:text-indigo-200 text-sm flex items-center gap-2 cursor-pointer" data-filter-type="showOnlyOwned">
        <span>Owned games</span>
        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" class="size-4 remove-filter" data-filter-type="showOnlyOwned">
          <path stroke-linecap="round" stroke-linejoin="round" d="M6 18 18 6M6 6l12 12" />
        </svg>
      </button>`);
    }

    if (advancedFilters.playerCount) {
      badges.push(`<button class="filter-badge px-3 py-1 rounded-full bg-indigo-100 dark:bg-indigo-900 text-indigo-800 dark:text-indigo-200 text-sm flex items-center gap-2 cursor-pointer" data-filter-type="playerCount">
        <span>${advancedFilters.playerCount} player${advancedFilters.playerCount > 1 ? 's' : ''}</span>
        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" class="size-4 remove-filter" data-filter-type="playerCount">
          <path stroke-linecap="round" stroke-linejoin="round" d="M6 18 18 6M6 6l12 12" />
        </svg>
      </button>`);
    }

    advancedFilters.priorities.forEach(priority => {
      const labels = ['', 'Must have', 'Love to have', 'Like to have', 'Thinking about it'];
      badges.push(`<button class="filter-badge px-3 py-1 rounded-full bg-indigo-100 dark:bg-indigo-900 text-indigo-800 dark:text-indigo-200 text-sm flex items-center gap-2 cursor-pointer" data-filter-type="priority" data-filter-value="${priority}">
        <span>${labels[priority]}</span>
        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" class="size-4 remove-filter" data-filter-type="priority" data-filter-value="${priority}">
          <path stroke-linecap="round" stroke-linejoin="round" d="M6 18 18 6M6 6l12 12" />
        </svg>
      </button>`);
    });

    advancedFilters.priceRanges.forEach(range => {
      const labels: Record<string, string> = {
        '0-20': 'Under $20',
        '20-40': '$20-$40',
        '40-60': '$40-$60',
        '60-80': '$60-$80',
        '80-999': '$80+'
      };
      badges.push(`<button class="filter-badge px-3 py-1 rounded-full bg-indigo-100 dark:bg-indigo-900 text-indigo-800 dark:text-indigo-200 text-sm flex items-center gap-2 cursor-pointer" data-filter-type="price" data-filter-value="${range}">
        <span>${labels[range]}</span>
        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" class="size-4 remove-filter" data-filter-type="price" data-filter-value="${range}">
          <path stroke-linecap="round" stroke-linejoin="round" d="M6 18 18 6M6 6l12 12" />
        </svg>
      </button>`);
    });

    advancedFilters.timeRanges.forEach(range => {
      const labels: Record<string, string> = {
        '0-30': 'Quick (<30min)',
        '30-60': 'Short (30-60min)',
        '60-120': 'Medium (60-120min)',
        '120-999': 'Long (120+min)'
      };
      badges.push(`<button class="filter-badge px-3 py-1 rounded-full bg-indigo-100 dark:bg-indigo-900 text-indigo-800 dark:text-indigo-200 text-sm flex items-center gap-2 cursor-pointer" data-filter-type="time" data-filter-value="${range}">
        <span>${labels[range]}</span>
        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" class="size-4 remove-filter" data-filter-type="time" data-filter-value="${range}">
          <path stroke-linecap="round" stroke-linejoin="round" d="M6 18 18 6M6 6l12 12" />
        </svg>
      </button>`);
    });

    if (badges.length > 0) {
      badgesContainer.innerHTML = badges.join('');
      badgesContainer.classList.remove('hidden');

      badgesContainer.querySelectorAll('.filter-badge').forEach(badge => {
        badge.addEventListener('click', (e) => {
          const target = e.target as HTMLElement;
          if (target.classList.contains('remove-filter')) {
            removeFilter(target.dataset.filterType || '', target.dataset.filterValue || '');
          } else {
            // Sync with modal
            if ((window as any).setFilterState) {
              (window as any).setFilterState(advancedFilters);
            }
            if ((window as any).openFilterModal) {
              (window as any).openFilterModal();
            }
          }
        });
      });
    } else {
      badgesContainer.classList.add('hidden');
    }
  }

  function removeFilter(type: string, value: string) {
    switch (type) {
      case 'showOnlyNew':
        advancedFilters.showOnlyNew = false;
        if (newFilterButton) {
          newFilterButton.dataset.active = 'false';
          newFilterButton.classList.remove('invert');
        }
        break;
      case 'showOnlyOwned':
        advancedFilters.showOnlyOwned = false;
        if (ownedButton) {
          ownedButton.dataset.active = 'false';
          ownedButton.classList.remove('dark:bg-green-300', 'dark:border-green-700', 'dark:text-green-700');
          ownedButton.classList.add('dark:text-white');
        }
        break;
      case 'playerCount':
        advancedFilters.playerCount = null;
        break;
      case 'priority':
        advancedFilters.priorities = advancedFilters.priorities.filter(p => p !== parseInt(value));
        break;
      case 'price':
        advancedFilters.priceRanges = advancedFilters.priceRanges.filter(r => r !== value);
        break;
      case 'time':
        advancedFilters.timeRanges = advancedFilters.timeRanges.filter(r => r !== value);
        break;
    }

    // Sync with modal
    if ((window as any).setFilterState) {
      (window as any).setFilterState(advancedFilters);
    }
    filterGames();
    updateFilterBadges();
    updateQueryParams();
  }

  // Function to filter games based on current filters
  function filterGames() {
    const searchTerm = searchInput?.value?.toLowerCase().trim() || "";

    games.forEach(game => {
      const gameName = game.dataset.name || "";
      const isNew = game.dataset.isNew === "true";
      const isUpdated = game.dataset.isUpdated === "true";
      const isOwned = game.dataset.owned === "true";
      const players = game.dataset.players || "";
      const playingTime = game.dataset.playtime || "";

      const matchesSearch = gameName.includes(searchTerm);
      const matchesNewFilter = !advancedFilters.showOnlyNew || (advancedFilters.showOnlyNew && (isNew || isUpdated));
      const matchesOwnedFilter = advancedFilters.showOnlyOwned ? isOwned : !isOwned;

      let matchesPlayerCount = true;
      if (advancedFilters.playerCount) {
        matchesPlayerCount = matchesPlayerCountRange(players, advancedFilters.playerCount);
      }

      let matchesPriority = true;
      if (advancedFilters.priorities.length > 0) {
        if (isOwned) {
          matchesPriority = false;
        } else {
          const gamePriority = parseInt(game.dataset.priority || '0');
          matchesPriority = advancedFilters.priorities.includes(gamePriority);
        }
      }

      let matchesPrice = true;
      if (advancedFilters.priceRanges.length > 0) {
        if (isOwned) {
          matchesPrice = false;
        } else {
          const priceStr = game.dataset.price;
          // Exclude games with no price when filtering by price
          if (!priceStr || priceStr === '0' || priceStr === 'NaN') {
            matchesPrice = false;
          } else {
            const gamePrice = parseFloat(priceStr);
            matchesPrice = matchesPriceRange(gamePrice, advancedFilters.priceRanges);
          }
        }
      }

      let matchesTime = true;
      if (advancedFilters.timeRanges.length > 0 && playingTime) {
        const time = parseInt(playingTime);
        matchesTime = matchesTimeRange(time, advancedFilters.timeRanges);
      }

      if (matchesSearch && matchesNewFilter && matchesOwnedFilter &&
          matchesPlayerCount && matchesPriority && matchesPrice && matchesTime) {
        game.style.display = "";
      } else {
        game.style.display = "none";
      }
    });

    // Update "no results" message
    if (noResultsMessageElement) {
      let message = "I'm not tracking any board games.";

      if (advancedFilters.showOnlyNew && advancedFilters.showOnlyOwned) {
        message = searchTerm
          ? `No games matching "${searchTerm}" that I own have been added in the last month.`
          : "No games that I own have been added in the last month.";
      } else if (advancedFilters.showOnlyNew) {
        message = searchTerm
          ? `No games matching "${searchTerm}" have been added in the last month.`
          : "No games have been added in the last month.";
      } else if (advancedFilters.showOnlyOwned) {
        message = searchTerm
          ? `I don't own any games matching "${searchTerm}".`
          : "I don't own any board games yet 😭";
      } else if (searchTerm) {
        message = `No games matching "${searchTerm}" are on my wishlist.`;
      }

      noResultsMessageElement.textContent = message;
    }

    const visibleGames = games.filter(game => game.style.display !== "none");
    noResultsElement?.classList.toggle("hidden", visibleGames.length > 0);

    if (gameCountElement) {
      gameCountElement.textContent = `Showing ${visibleGames.length}`;
      if (advancedFilters.showOnlyNew) {
        gameCountElement.textContent += " recently added";
      }
      gameCountElement.textContent += ` game${visibleGames.length !== 1 ? "s" : ""}`;
      if (advancedFilters.showOnlyOwned) {
        gameCountElement.textContent += " that I own";
      }
      gameCountElement.textContent += ".";
    }
  }

  searchInput?.addEventListener("input", () => {
    filterGames();
    updateQueryParams();
  });

  newFilterButton?.addEventListener("click", () => {
    const currentlyActive = newFilterButton.dataset.active === "true";
    advancedFilters.showOnlyNew = !currentlyActive;
    newFilterButton.dataset.active = (!currentlyActive).toString();
    newFilterButton.classList.toggle("invert");

    // Sync with modal
    if ((window as any).setFilterState) {
      (window as any).setFilterState(advancedFilters);
    }

    updateQueryParams();
    filterGames();
    updateFilterBadges();
  });

  ownedButton?.addEventListener("click", () => {
    const currentlyActive = ownedButton.dataset.active === "true";
    advancedFilters.showOnlyOwned = !currentlyActive;
    ownedButton.dataset.active = (!currentlyActive).toString();
    ownedButton.classList.toggle("dark:bg-green-300");
    ownedButton.classList.toggle("dark:border-green-700");
    ownedButton.classList.toggle("dark:text-green-700");
    ownedButton.classList.toggle("dark:text-white");

    // Sync with modal
    if ((window as any).setFilterState) {
      (window as any).setFilterState(advancedFilters);
    }

    updateQueryParams();
    filterGames();
    updateFilterBadges();
  });

  window.addEventListener("popstate", () => {
    // Reapply query params and filter games when navigating back/forward
    applyQueryParams();
    filterGames();
  });

  // Initial filter application
  applyQueryParams();
  filterGames();
  updateFilterBadges();

  games.forEach((card) => {
    card.addEventListener('click', () => {
      const wishlistDataElement = document.getElementById('wishlist-data');
      if (!wishlistDataElement) return;
      
      const wishlistData = JSON.parse(wishlistDataElement.textContent || '[]');
      
      const gameName = card.dataset.name;
      const game = wishlistData.find((g: any) => g.name.toLowerCase() === gameName);
      
      if (game && (window as any).openGameModal) {
        (window as any).openGameModal(game);
      }
    });

    card.style.cursor = 'pointer';
  });

  const openFiltersButton = document.getElementById('open-filters');
  openFiltersButton?.addEventListener('click', () => {
    // Set current filter state in modal before opening
    if ((window as any).setFilterState) {
      (window as any).setFilterState(advancedFilters);
    }
    if ((window as any).openFilterModal) {
      (window as any).openFilterModal();
    }
  });

  // Register callback for when filters change in the modal
  if ((window as any).setFiltersChangeCallback) {
    (window as any).setFiltersChangeCallback(() => {
      // Get updated filter state from modal
      if ((window as any).getFilterState) {
        advancedFilters = (window as any).getFilterState();
        
        // Sync quick filter button states
        if (newFilterButton) {
          newFilterButton.dataset.active = advancedFilters.showOnlyNew.toString();
          newFilterButton.classList.toggle("invert", advancedFilters.showOnlyNew);
        }
        if (ownedButton) {
          ownedButton.dataset.active = advancedFilters.showOnlyOwned.toString();
          ownedButton.classList.toggle("dark:bg-green-300", advancedFilters.showOnlyOwned);
          ownedButton.classList.toggle("dark:border-green-700", advancedFilters.showOnlyOwned);
          ownedButton.classList.toggle("dark:text-green-700", advancedFilters.showOnlyOwned);
          ownedButton.classList.toggle("dark:text-white", !advancedFilters.showOnlyOwned);
        }
        
        filterGames();
        updateFilterBadges();
        updateQueryParams();
      }
    });
  }
