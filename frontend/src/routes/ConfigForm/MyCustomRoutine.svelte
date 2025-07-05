<script lang="ts">
  import IconX from "../Icons/Feather/IconX.svelte";
  import IconArrowUp from "../Icons/Feather/IconArrowUp.svelte";
  import IconArrowDown from "../Icons/Feather/IconArrowDown.svelte";

  let {
    constraint,
    value = $bindable(),
    name,
  }: {
    constraint: MyCustomRoutineConstraint;
    value: string[];
    name: string;
  } = $props();

  let draggedItem = $state<string | null>(null);
  let draggedFromSelected = $state(false);

  // Index where the placeholder (drop target) should appear (0..value.length)
  let insertIndex = $state(-1);
  let placeholderTop = $state(-1); // px offset of overlay bar within selected container

  // Which list is currently being hovered during drag (for visual highlight)
  let overList = $state<"none" | "available" | "selected">("none");

  // --- Selection state ---
  let selectedAvailable = $state<Set<string>>(new Set());
  let selectedChosen = $state<Set<number>>(new Set());

  function toggleAvailableSelection(task: string) {
    selectedChosen = new Set();
    const newSet = new Set(selectedAvailable);
    if (newSet.has(task)) newSet.delete(task);
    else newSet.add(task);
    selectedAvailable = newSet;
  }

  function toggleChosenSelection(index: number) {
    selectedAvailable = new Set();
    const newSet = new Set(selectedChosen);
    if (newSet.has(index)) newSet.delete(index);
    else newSet.add(index);
    selectedChosen = newSet;
  }

  function moveSelectedToChosen() {
    if (selectedAvailable.size === 0) return;
    const tasksToAdd: string[] = Array.from(selectedAvailable);
    value = [...value, ...tasksToAdd];
    // reset selections
    selectedAvailable = new Set();
  }

  function moveChosen(offset: -1 | 1) {
    if (selectedChosen.size === 0) return;

    const indices = Array.from(selectedChosen).sort((a, b) => a - b);
    if (offset === -1 && indices[0] === 0) return;
    if (offset === 1 && indices[indices.length - 1] === value.length - 1)
      return;

    // Ensure correct swap order (top-to-bottom for ↑, bottom-to-top for ↓)
    const ordered = offset === -1 ? indices : [...indices].reverse();
    const newArr = [...value];

    for (const idx of ordered) {
      [newArr[idx], newArr[idx + offset]] = [newArr[idx + offset], newArr[idx]];
    }

    value = newArr;
    selectedChosen = new Set(indices.map((i) => i + offset));
  }

  function handleDragStart(
    e: DragEvent,
    task: string,
    fromSelected: boolean,
    index: number = -1,
  ) {
    draggedItem = task;
    draggedFromSelected = fromSelected;

    if (fromSelected) {
      // If dragging an unselected item, make it the only selection
      if (!selectedChosen.has(index)) {
        selectedChosen = new Set([index]);
      }
    } else {
      // If dragging an unselected item, make it the only selection
      if (!selectedAvailable.has(task)) {
        selectedAvailable = new Set([task]);
      }
    }

    if (e.dataTransfer) {
      e.dataTransfer.effectAllowed = fromSelected ? "move" : "copy";
    }
  }

  function handleDragOver(e: DragEvent) {
    e.preventDefault();
    if (e.dataTransfer) {
      e.dataTransfer.dropEffect = draggedFromSelected ? "move" : "copy";
    }
    overList = "none";
  }

  function handleContainerDragOver(
    e: DragEvent,
    list: "available" | "selected",
  ) {
    handleDragOver(e);
    overList = list;

    // For selected list we calculate precise insertIndex even when hovering gaps
    if (list === "selected") {
      const container = e.currentTarget as HTMLElement;
      const children = Array.from(
        container.querySelectorAll<HTMLElement>("[data-idx]"),
      );

      // Default to end
      let newIndex = children.length;
      let newTop = container.scrollHeight; // default bottom
      for (const child of children) {
        const rect = child.getBoundingClientRect();
        const idx = parseInt(child.dataset.idx!);
        const upper = rect.top + rect.height * 0.35;
        if (e.clientY < upper) {
          newIndex = idx;
          newTop = rect.top - container.getBoundingClientRect().top;
          break;
        }
      }
      insertIndex = newIndex;
      // Clamp to container bounds to avoid bar going outside
      const maxTop = container.clientHeight - 2; // bar height ~2px
      placeholderTop = Math.min(Math.max(0, newTop), maxTop);
    }
  }

  function handleContainerDragLeave() {
    overList = "none";
    insertIndex = -1; // Reset drop indicator
    placeholderTop = -1;
  }

  // Drop on the available list removes the task from the selected list (if dragged from there)
  function handleDropOnAvailable(e: DragEvent) {
    e.preventDefault();
    if (!draggedItem || !draggedFromSelected) return;

    // Remove all items that were in selectedChosen
    const newSelectedValue = value.filter((_, i) => !selectedChosen.has(i));
    value = newSelectedValue;
    selectedChosen = new Set(); // Clear selection

    // reset state
    draggedItem = null;
    draggedFromSelected = false;
    overList = "none";
  }

  function handleDrop(e: DragEvent, targetIndex?: number) {
    e.preventDefault();
    if (!draggedItem) return; // Safety guard

    if (draggedFromSelected) {
      // --- Moving (reordering) within selected tasks ---
      const indicesToMove = [...selectedChosen].sort((a, b) => a - b);
      if (indicesToMove.length === 0) return;

      const tasksToMove = indicesToMove.map((i) => value[i]);
      const remainingTasks = value.filter((_, i) => !selectedChosen.has(i));

      const dropIndex = targetIndex !== undefined ? targetIndex : insertIndex;
      if (dropIndex === -1) {
        // This can happen if dropping on the container but not on a specific item,
        // it should append to the end of the list.
        value = [...remainingTasks, ...tasksToMove];
        const newSelection = new Set<number>(
          tasksToMove.map((_, idx) => remainingTasks.length + idx),
        );
        selectedChosen = newSelection;
      } else {
        // Adjust drop index for the filtered array
        const numMovedBeforeDrop = indicesToMove.filter(
          (i) => i < dropIndex,
        ).length;
        const adjustedDropIndex = dropIndex - numMovedBeforeDrop;

        // Insert the tasks
        remainingTasks.splice(adjustedDropIndex, 0, ...tasksToMove);
        value = remainingTasks;

        // Update selection to the new positions of moved items
        const newSelection = new Set<number>(
          tasksToMove.map((_, idx) => adjustedDropIndex + idx),
        );
        selectedChosen = newSelection;
      }
    } else {
      // --- Adding from available tasks ---
      const tasksToAdd = [...selectedAvailable];
      if (tasksToAdd.length === 0) return;

      const insertAt =
        targetIndex !== undefined
          ? targetIndex
          : insertIndex !== -1
            ? insertIndex
            : value.length;

      const newValue = [...value];
      newValue.splice(insertAt, 0, ...tasksToAdd);
      value = newValue;

      // Update selection to the new positions of added items
      const newSelection = new Set<number>(
        tasksToAdd.map((_, idx) => insertAt + idx),
      );
      selectedChosen = newSelection;

      // Clear the selection from the available list
      selectedAvailable = new Set();
    }

    // Reset visual helpers and drag state AFTER the drop is handled
    draggedItem = null;
    draggedFromSelected = false;
    overList = "none";
    insertIndex = -1;
    placeholderTop = -1;
  }

  function removeTask(index: number) {
    value = value.filter((_, i) => i !== index);

    selectedChosen = new Set(
      [...selectedChosen]
        .filter((i) => i !== index) // drop the removed item if it was selected
        .map((i) => (i > index ? i - 1 : i)), // shift indices after the removed spot
    );
  }

  function clearList() {
    if (confirm("Are you sure you want to clear all tasks?")) {
      value = [];
      selectedChosen = new Set();
    }
  }

  function addTask(task: string) {
    value = [...value, task];
  }

  let taskHeader = $state("Tasks");
  let taskBracketInfo = $state("");
  let taskDescription = $state(
    "These actions will run in the order shown below.",
  );

  const lowerName = name.toLowerCase();

  if (lowerName.includes("daily")) {
    taskHeader = "Daily Tasks";
    taskBracketInfo = "(Run once per day)";
    taskDescription = "These actions will run once at the start of each day.";
  } else if (lowerName.includes("repeat")) {
    taskHeader = "Repeating Tasks";
    taskBracketInfo = "(Run continuously)";
    taskDescription =
      "These actions will run repeatedly in order, over and over again.";
  }
</script>

<div class="mx-auto flex w-full flex-col gap-4 p-4">
  {#if constraint.choices.length > 0}
    <div>
      <div class="flex items-center justify-between">
        <div class="flex items-center gap-4">
          <h6 class="h6">{taskHeader}</h6>
          <span class="">{taskBracketInfo}</span>
        </div>
        <button
          class="btn preset-filled-warning-100-900 hover:preset-filled-warning-500"
          type="button"
          onclick={clearList}>Clear List</button
        >
      </div>
      <p>{taskDescription}</p>

      <div
        class="mt-4 grid grid-cols-1 gap-6 lg:grid-cols-[1fr_auto_1fr] lg:items-start"
      >
        <!-- Available Tasks Panel -->
        <div class="flex flex-col">
          <div class="mb-3 flex items-center justify-between">
            <h6 class="text-surface-600-300 text-sm font-semibold">
              Available Actions (Drag or double-click to add)
            </h6>
            <!-- invisible placeholder to match height of Up/Down buttons on right panel -->
            <div class="invisible flex gap-1">
              <button class="btn-icon preset-filled-secondary-100-900"
                ><IconArrowUp size={16} /></button
              >
              <button class="btn-icon preset-filled-secondary-100-900"
                ><IconArrowDown size={16} /></button
              >
            </div>
          </div>
          <div
            class="bg-surface-50-900 flex min-h-[200px] flex-col gap-2 rounded-lg border border-white/20 p-3"
            ondragover={(e) => handleContainerDragOver(e, "available")}
            ondragleave={handleContainerDragLeave}
            ondrop={handleDropOnAvailable}
            class:ring-2={overList === "available"}
            class:ring-secondary-400={overList === "available"}
            role="list"
          >
            {#if constraint.choices.length === 0}
              <p class="text-surface-400-500 text-center text-sm">
                No actions available
              </p>
            {:else}
              {#each constraint.choices as task}
                <div
                  class="bg-surface-100-800 cursor-grab rounded-md p-3 shadow-sm ring-offset-2 ring-offset-surface-900 transition-all duration-150 hover:shadow-md active:cursor-grabbing"
                  class:ring-2={selectedAvailable.has(task)}
                  class:ring-secondary-400={selectedAvailable.has(task)}
                  draggable="true"
                  ondragstart={(e) => handleDragStart(e, task, false)}
                  ondblclick={() => addTask(task)}
                  onclick={() => toggleAvailableSelection(task)}
                  role="button"
                  tabindex="0"
                  title="Double-click to add, or drag to position"
                  onkeydown={(e) => {
                    if (e.key === " " || e.key === "Enter") {
                      e.preventDefault();
                      toggleAvailableSelection(task);
                    }
                  }}
                  aria-grabbed={selectedAvailable.has(task) ? "true" : "false"}
                >
                  <p class="text-sm">{task}</p>
                </div>
              {/each}
            {/if}
          </div>
        </div>

        <!-- Arrow Column -->
        <div class="flex items-start justify-center pt-8">
          <button
            class="btn preset-filled-secondary-100-900 hover:preset-filled-secondary-500"
            type="button"
            title="Add selected"
            onclick={moveSelectedToChosen}
            disabled={selectedAvailable.size === 0}
          >
            ➔
          </button>
        </div>

        <!-- Selected Tasks Panel -->
        <div class="flex flex-col">
          <div class="mb-3 flex items-center justify-between">
            <h6 class="text-surface-600-300 text-sm font-semibold">
              Selected Actions (Drag to reorder)
            </h6>
            <div class="flex gap-1">
              <button
                class="btn-icon preset-filled-secondary-100-900 transition-transform duration-150 hover:-translate-y-0.5"
                type="button"
                title="Move up"
                onclick={() => moveChosen(-1)}
                disabled={selectedChosen.size === 0}
              >
                <IconArrowUp size={16} />
              </button>
              <button
                class="btn-icon preset-filled-secondary-100-900 transition-transform duration-150 hover:translate-y-0.5"
                type="button"
                title="Move down"
                onclick={() => moveChosen(1)}
                disabled={selectedChosen.size === 0}
              >
                <IconArrowDown size={16} />
              </button>
            </div>
          </div>
          <!-- svelte-ignore a11y_no_static_element_interactions -->
          <div
            class="bg-primary-50-900/10 relative flex min-h-[200px] flex-col gap-2 rounded-lg border border-white/20 p-3"
            ondragover={(e) => handleContainerDragOver(e, "selected")}
            ondragleave={handleContainerDragLeave}
            ondrop={(e) => handleDrop(e)}
            class:ring-2={overList === "selected"}
            class:ring-primary-400={overList === "selected"}
            role="list"
          >
            <!-- overlay placeholder bar -->
            {#if draggedItem && placeholderTop !== -1}
              <div
                class="pointer-events-none absolute right-0 left-0 h-1.5 rounded bg-primary-400/80 transition-all duration-75"
                style={`top: ${placeholderTop}px;`}
              ></div>
            {/if}
            {#if value.length === 0}
              <p class="text-surface-400-500 text-center text-sm">
                Drag actions here to add them
              </p>
            {:else}
              {#each value as task, index}
                <div
                  data-idx={index}
                  class="group bg-primary-100-800 relative cursor-grab rounded-md p-3 shadow-sm ring-offset-2 ring-offset-surface-900 transition-transform duration-150 hover:shadow-md active:cursor-grabbing"
                  class:ring-2={(draggedItem === task && draggedFromSelected) ||
                    selectedChosen.has(index)}
                  class:ring-primary-400={(draggedItem === task &&
                    draggedFromSelected) ||
                    selectedChosen.has(index)}
                  draggable="true"
                  ondragstart={(e) => handleDragStart(e, task, true, index)}
                  ondragover={(e) => {
                    e.preventDefault();
                  }}
                  onclick={() => toggleChosenSelection(index)}
                  aria-grabbed={selectedChosen.has(index) ||
                  (draggedItem === task && draggedFromSelected)
                    ? "true"
                    : "false"}
                  role="button"
                  tabindex="0"
                  onkeydown={(e) => {
                    if (e.key === " " || e.key === "Enter") {
                      e.preventDefault();
                      toggleChosenSelection(index);
                    }
                  }}
                >
                  <div
                    class="flex items-center justify-between gap-2 select-none"
                  >
                    <div class="flex items-center gap-2">
                      <span
                        class="text-surface-500-400 text-base font-semibold"
                      >
                        {index + 1}.
                      </span>
                      <p class="text-sm">{task}</p>
                    </div>
                    <button
                      class="badge-icon preset-filled-error-100-900 opacity-0 transition-opacity group-hover:opacity-100 hover:preset-filled-error-500"
                      type="button"
                      onclick={(e) => {
                        e.stopPropagation();
                        removeTask(index);
                      }}
                    >
                      <IconX size={16} />
                    </button>
                  </div>
                  <input type="hidden" {name} value={task} />
                </div>
              {/each}
            {/if}
          </div>
        </div>
      </div>
    </div>
  {:else}
    <p>No options available</p>
  {/if}
</div>
