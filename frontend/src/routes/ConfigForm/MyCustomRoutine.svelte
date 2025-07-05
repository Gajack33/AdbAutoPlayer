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
  let draggedIndex = $state(-1);

  // Index where the placeholder (drop target) should appear (0..value.length)
  let insertIndex = $state(-1);

  // Which list is currently being hovered during drag (for visual highlight)
  let overList = $state<"none" | "available" | "selected">("none");

  // --- Selection state ---
  let selectedAvailable = $state<Set<string>>(new Set());
  let selectedChosen = $state<Set<string>>(new Set());

  function toggleAvailableSelection(task: string) {
    selectedChosen = new Set();
    const newSet = new Set(selectedAvailable);
    if (newSet.has(task)) newSet.delete(task);
    else newSet.add(task);
    selectedAvailable = newSet;
  }

  function toggleChosenSelection(task: string) {
    selectedAvailable = new Set();
    const newSet = new Set(selectedChosen);
    if (newSet.has(task)) newSet.delete(task);
    else newSet.add(task);
    selectedChosen = newSet;
  }

  function moveSelectedToChosen() {
    if (selectedAvailable.size === 0) return;
    const tasksToAdd = Array.from(selectedAvailable).filter(
      (t) => !value.includes(t),
    );
    value = [...value, ...tasksToAdd];
    // reset selections
    selectedAvailable = new Set();
  }

  function moveChosenUp() {
    if (selectedChosen.size === 0) return;
    const indices = value
      .map((task, i) => (selectedChosen.has(task) ? i : -1))
      .filter((i) => i !== -1)
      .sort((a, b) => a - b);
    if (indices[0] === 0) return;
    const newArr = [...value];
    for (const idx of indices) {
      [newArr[idx - 1], newArr[idx]] = [newArr[idx], newArr[idx - 1]];
    }
    value = newArr;
  }

  function moveChosenDown() {
    if (selectedChosen.size === 0) return;
    const indices = value
      .map((task, i) => (selectedChosen.has(task) ? i : -1))
      .filter((i) => i !== -1)
      .sort((a, b) => b - a);
    if (indices[0] === value.length - 1) return;
    const newArr = [...value];
    for (const idx of indices) {
      [newArr[idx], newArr[idx + 1]] = [newArr[idx + 1], newArr[idx]];
    }
    value = newArr;
  }

  function handleDragStart(
    e: DragEvent,
    task: string,
    fromSelected: boolean,
    index: number = -1,
  ) {
    draggedItem = task;
    draggedFromSelected = fromSelected;
    draggedIndex = index;
    if (e.dataTransfer) {
      e.dataTransfer.effectAllowed = fromSelected ? "move" : "copy";
    }
  }

  function handleDragOver(e: DragEvent) {
    e.preventDefault();
    if (e.dataTransfer) {
      e.dataTransfer.dropEffect = draggedFromSelected ? "move" : "copy";
    }
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
      for (const child of children) {
        const rect = child.getBoundingClientRect();
        const midY = rect.top + rect.height / 2;
        if (e.clientY < midY) {
          newIndex = parseInt(child.dataset.idx!);
          break;
        }
      }
      insertIndex = newIndex;
    }
  }

  function handleContainerDragLeave() {
    overList = "none";
  }

  // Drop on the available list removes the task from the selected list (if dragged from there)
  function handleDropOnAvailable(e: DragEvent) {
    e.preventDefault();
    if (!draggedItem) return;

    if (draggedFromSelected) {
      value = value.filter((item) => item !== draggedItem);
    }

    // reset state
    draggedItem = null;
    draggedFromSelected = false;
    draggedIndex = -1;
    overList = "none";
  }

  function handleDrop(e: DragEvent, targetIndex?: number) {
    e.preventDefault();
    if (!draggedItem) return;

    if (draggedFromSelected) {
      // Moving within selected tasks
      const target = targetIndex !== undefined ? targetIndex : insertIndex;
      if (target !== -1 && draggedIndex !== -1) {
        let newValue = [...value];
        const [movedItem] = newValue.splice(draggedIndex, 1);
        // Adjust target index if original item was before desired spot
        const adjustedIndex = draggedIndex < target ? target - 1 : target;
        newValue.splice(adjustedIndex, 0, movedItem);
        value = newValue;
      }
    } else {
      // Adding from available tasks
      // Prevent duplicates
      if (value.includes(draggedItem)) {
        draggedItem = null;
        draggedFromSelected = false;
        draggedIndex = -1;
        insertIndex = -1;
        overList = "none";
        return;
      }

      const insertAt =
        targetIndex !== undefined
          ? targetIndex
          : insertIndex !== -1
            ? insertIndex
            : value.length;
      const newValue = [...value];
      newValue.splice(insertAt, 0, draggedItem);
      value = newValue;
    }

    // Reset visual helpers and drag state AFTER the drop is handled
    draggedItem = null;
    draggedFromSelected = false;
    draggedIndex = -1;
    insertIndex = -1;
    overList = "none";
  }

  function removeTask(index: number) {
    value = value.filter((_, i) => i !== index);
  }

  function clearList() {
    if (confirm("Are you sure you want to clear all tasks?")) {
      value = [];
    }
  }

  function addTask(task: string) {
    if (value.includes(task)) return; // avoid duplicates
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
          <h6 class="text-surface-600-300 mb-3 text-sm font-semibold">
            Available Actions (Drag or double-click to add)
          </h6>
          <div
            class="border-surface-300-600 bg-surface-50-900 flex min-h-[200px] flex-col gap-2 rounded-lg border-2 border-dashed p-3"
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
                  class="bg-surface-100-800 cursor-grab rounded-md p-3 shadow-sm transition-all hover:shadow-md active:cursor-grabbing"
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
                class="badge-icon preset-filled-secondary-100-900"
                type="button"
                title="Move up"
                onclick={moveChosenUp}
                disabled={selectedChosen.size === 0}
              >
                <IconArrowUp size={16} />
              </button>
              <button
                class="badge-icon preset-filled-secondary-100-900"
                type="button"
                title="Move down"
                onclick={moveChosenDown}
                disabled={selectedChosen.size === 0}
              >
                <IconArrowDown size={16} />
              </button>
            </div>
          </div>
          <!-- svelte-ignore a11y_no_static_element_interactions -->
          <div
            class="border-primary-300-600 bg-primary-50-900/20 flex min-h-[200px] flex-col gap-2 rounded-lg border-2 border-dashed p-3"
            ondragover={(e) => handleContainerDragOver(e, "selected")}
            ondragleave={handleContainerDragLeave}
            ondrop={(e) => handleDrop(e)}
            class:ring-2={overList === "selected"}
            class:ring-primary-400={overList === "selected"}
            role="list"
          >
            {#if value.length === 0}
              <p class="text-surface-400-500 text-center text-sm">
                Drag actions here to add them
              </p>
            {:else}
              {#each value as task, index}
                {#if insertIndex === index}
                  <!-- Placeholder line indicating drop position -->
                  <div class="h-2 w-full rounded bg-primary-500/50"></div>
                {/if}
                <div
                  data-idx={index}
                  class="group bg-primary-100-800 relative cursor-grab rounded-md p-3 shadow-sm transition-all hover:shadow-md active:cursor-grabbing"
                  class:ring-2={(draggedItem === task && draggedFromSelected) ||
                    selectedChosen.has(task)}
                  class:ring-primary-400={(draggedItem === task &&
                    draggedFromSelected) ||
                    selectedChosen.has(task)}
                  draggable="true"
                  ondragstart={(e) => handleDragStart(e, task, true, index)}
                  ondragover={(e) => {
                    e.preventDefault();
                    e.stopPropagation();
                    const target = e.currentTarget as HTMLElement;
                    const rect = target.getBoundingClientRect();
                    const offsetY = e.clientY - rect.top;
                    insertIndex = offsetY < rect.height / 2 ? index : index + 1;
                  }}
                  onclick={() => toggleChosenSelection(task)}
                  role="button"
                  tabindex="0"
                  onkeydown={(e) => {
                    if (e.key === " " || e.key === "Enter") {
                      e.preventDefault();
                      toggleChosenSelection(task);
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
                      onclick={() => removeTask(index)}
                    >
                      <IconX size={16} />
                    </button>
                  </div>
                  <input type="hidden" {name} value={task} />
                </div>
              {/each}
              {#if insertIndex === value.length}
                <!-- Placeholder at end of list -->
                <div class="h-2 w-full rounded bg-primary-500/50"></div>
              {/if}
            {/if}
          </div>
        </div>
      </div>
    </div>
  {:else}
    <p>No options available</p>
  {/if}
</div>
